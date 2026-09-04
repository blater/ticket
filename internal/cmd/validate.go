package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/radutopala/ticket/internal/gitmeta"
	tk "github.com/radutopala/ticket/pkg/ticket"
)

var validateFlags struct {
	commits string
	ticket  string
}

type validationIssue struct {
	TicketID string `json:"ticket_id,omitempty"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

type validationReport struct {
	Valid  bool              `json:"valid"`
	Count  int               `json:"count"`
	Issues []validationIssue `json:"issues"`
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate ticket graph and delivery links",
	Long: `Validate ticket identities, types, parents, dependencies, links,
cycles, and configured Git delivery requirements. The command does not mutate
tickets or Git state.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		tickets, err := store.List()
		if err != nil {
			return err
		}

		issues := validateTickets(tickets)
		if validateFlags.ticket != "" && validateFlags.commits == "" {
			return fmt.Errorf("--ticket requires --commits")
		}
		if validateFlags.commits != "" {
			commits, err := gitmeta.New("").CommitsWithTrailers(
				validateFlags.commits,
				cfg.Delivery.CommitTrailer,
			)
			if err != nil {
				return err
			}
			ticketIDs := make(map[string]struct{}, len(tickets))
			for _, ticket := range tickets {
				ticketIDs[ticket.ID] = struct{}{}
			}
			var requiredTicketID string
			if validateFlags.ticket != "" {
				requiredTicketID, err = store.ResolveID(validateFlags.ticket)
				if err != nil {
					return err
				}
			}
			for _, commit := range commits {
				for _, ticketID := range commit.Values {
					if _, exists := ticketIDs[ticketID]; !exists {
						issues = append(issues, validationIssue{
							Code:    "missing-ticket",
							Message: fmt.Sprintf("commit %s references %s", commit.Commit, ticketID),
						})
					}
				}
				if requiredTicketID != "" && len(commit.Parents) <= 1 && !contains(commit.Values, requiredTicketID) {
					issues = append(issues, validationIssue{
						TicketID: requiredTicketID,
						Code:     "missing-commit-trailer",
						Message:  fmt.Sprintf("commit %s lacks %s: %s", commit.Commit, cfg.Delivery.CommitTrailer, requiredTicketID),
					})
				}
			}
		}
		report := validationReport{
			Valid:  len(issues) == 0,
			Count:  len(issues),
			Issues: issues,
		}
		if jsonOutput {
			if err := outputJSON(cmd, report); err != nil {
				return err
			}
		} else if report.Valid {
			fmt.Println("tickets=valid")
		} else {
			for _, issue := range report.Issues {
				if issue.TicketID == "" {
					fmt.Printf("%s: %s\n", issue.Code, issue.Message)
				} else {
					fmt.Printf("%s %s: %s\n", issue.TicketID, issue.Code, issue.Message)
				}
			}
		}

		if !report.Valid {
			return fmt.Errorf("ticket validation failed with %d issue(s)", report.Count)
		}
		return nil
	},
}

func init() {
	validateCmd.Flags().StringVar(&validateFlags.commits, "commits", "", "Git revision or range whose ticket trailers must resolve")
	validateCmd.Flags().StringVar(&validateFlags.ticket, "ticket", "", "Require every non-merge commit in --commits to reference this ticket")
}

func validateTickets(tickets []*tk.Ticket) []validationIssue {
	var issues []validationIssue
	ticketMap := make(map[string]*tk.Ticket, len(tickets))
	for _, ticket := range tickets {
		if previous, exists := ticketMap[ticket.ID]; exists {
			issues = append(issues, validationIssue{
				TicketID: ticket.ID,
				Code:     "duplicate-id",
				Message:  fmt.Sprintf("also used by %q", previous.Title),
			})
			continue
		}
		ticketMap[ticket.ID] = ticket
	}

	for _, ticket := range tickets {
		if ticket.ID == "" {
			issues = append(issues, validationIssue{Code: "missing-id", Message: "ticket has no id"})
		}
		if !ticket.Status.IsValid() {
			issues = append(issues, issue(ticket, "invalid-status", fmt.Sprintf("invalid status %q", ticket.Status)))
		}
		if !ticket.Type.IsValid() {
			issues = append(issues, issue(ticket, "invalid-type", fmt.Sprintf("invalid type %q", ticket.Type)))
		}
		if err := tk.ValidatePriority(ticket.Priority); err != nil {
			issues = append(issues, issue(ticket, "invalid-priority", err.Error()))
		}

		if ticket.Parent != "" {
			parent, exists := ticketMap[ticket.Parent]
			if !exists {
				issues = append(issues, issue(ticket, "missing-parent", ticket.Parent))
			} else if parent.Type != tk.TypeEpic {
				issues = append(issues, issue(ticket, "invalid-parent", fmt.Sprintf("%s is not an epic", ticket.Parent)))
			}
		}

		for _, dependency := range ticket.Deps {
			blocker, exists := ticketMap[dependency]
			if !exists {
				issues = append(issues, issue(ticket, "missing-dependency", dependency))
			} else if ticket.Status == tk.StatusClosed && blocker.Status != tk.StatusClosed {
				issues = append(issues, issue(ticket, "unresolved-dependency", dependency))
			}
		}

		for _, link := range ticket.Links {
			linked, exists := ticketMap[link]
			if !exists {
				issues = append(issues, issue(ticket, "missing-link", link))
			} else if !contains(linked.Links, ticket.ID) {
				issues = append(issues, issue(ticket, "asymmetric-link", link))
			}
		}

		for _, message := range validateTicketDelivery(ticket, ticket.Status == tk.StatusClosed) {
			issues = append(issues, issue(ticket, "invalid-delivery", message))
		}
	}

	for _, cycle := range tk.DetectCycles(tickets) {
		issues = append(issues, validationIssue{
			Code:    "dependency-cycle",
			Message: strings.Join(cycle, " -> "),
		})
	}

	for _, epic := range tickets {
		if epic.Type != tk.TypeEpic || epic.Status != tk.StatusClosed {
			continue
		}
		for _, child := range tickets {
			if child.Parent == epic.ID && child.Status != tk.StatusClosed {
				issues = append(issues, issue(epic, "open-child", child.ID))
			}
		}

	}

	sort.Slice(issues, func(i, j int) bool {
		if issues[i].TicketID != issues[j].TicketID {
			return issues[i].TicketID < issues[j].TicketID
		}
		if issues[i].Code != issues[j].Code {
			return issues[i].Code < issues[j].Code
		}
		return issues[i].Message < issues[j].Message
	})
	return issues
}

func issue(ticket *tk.Ticket, code, message string) validationIssue {
	return validationIssue{TicketID: ticket.ID, Code: code, Message: message}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
