package cmd

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	tk "github.com/radutopala/ticket/pkg/ticket"
)

var createFlags struct {
	description string
	design      string
	acceptance  string
	strategy    string
	ticketType  string
	priority    int
	assignee    string
	externalRef string
	pr          string
	delivery    string
	parent      string
	tags        []string
}

var createCmd = &cobra.Command{
	Use:   "create [title]",
	Short: "Create a new ticket using the configured ID strategy",
	Long: `Create a new ticket with the specified title and options.

Ticket IDs use the hexadecimal strategy by default unless ticket.yaml sets a project
strategy. Use --strategy to override it for this command.
Valid strategies are default, tolkien, hex, base32, and ulid.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		strategy, err := resolveCreateStrategy(cmd)
		if err != nil {
			return err
		}
		if err := tk.ValidatePriority(createFlags.priority); err != nil {
			return err
		}

		// Validate parent exists if specified
		if createFlags.parent != "" {
			resolvedParent, err := store.ResolveID(createFlags.parent)
			if err != nil {
				return fmt.Errorf("parent ticket not found: %s", createFlags.parent)
			}
			createFlags.parent = resolvedParent
		}

		assignee := createFlags.assignee
		if assignee == "" {
			assignee = getGitUserName()
		}

		ticket := &tk.Ticket{
			Status:      tk.StatusOpen,
			Priority:    createFlags.priority,
			Assignee:    assignee,
			ExternalRef: createFlags.externalRef,
			PR:          createFlags.pr,
			Parent:      createFlags.parent,
			Tags:        createFlags.tags,
			Created:     time.Now().UTC(),
			Description: createFlags.description,
			Design:      createFlags.design,
			Acceptance:  createFlags.acceptance,
		}

		if len(args) > 0 {
			ticket.Title = args[0]
		}

		if createFlags.ticketType != "" {
			t, err := tk.ParseType(createFlags.ticketType)
			if err != nil {
				return err
			}
			ticket.Type = t
		} else {
			ticket.Type = tk.TypeTask
		}

		if createFlags.delivery != "" {
			delivery, err := tk.ParseDelivery(createFlags.delivery)
			if err != nil {
				return err
			}
			ticket.Delivery = delivery
		} else {
			ticket.Delivery = tk.DefaultDelivery(ticket.Type)
		}
		if ticket.Delivery == tk.DeliveryNone && ticket.Type != tk.TypeEpic {
			return fmt.Errorf("delivery none is valid only for epics")
		}

		if err := store.CreateWithStrategy(ticket, strategy); err != nil {
			return fmt.Errorf("failed to create ticket: %w", err)
		}

		if jsonOutput {
			return outputJSON(cmd, ticket)
		}

		fmt.Println(ticket.ID)
		return nil
	},
}

func resolveCreateStrategy(cmd *cobra.Command) (tk.IDStrategy, error) {
	strategy := string(tk.IDStrategyHex)
	if cfg != nil && cfg.Strategy != "" {
		strategy = cfg.Strategy
	}
	if cmd.Flags().Changed("strategy") {
		strategy = createFlags.strategy
	}
	return tk.ParseIDStrategy(strategy)
}

// getGitUserName returns the git user.name config value, or empty string if unavailable.
func getGitUserName() string {
	cmd := exec.Command("git", "config", "user.name")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func init() {
	createCmd.Flags().StringVarP(&createFlags.description, "description", "d", "", "Description text")
	createCmd.Flags().StringVarP(&createFlags.strategy, "strategy", "s", "", "ID strategy (default|tolkien|hex|base32|ulid; overrides ticket.yaml)")
	createCmd.Flags().StringVar(&createFlags.design, "design", "", "Design notes")
	createCmd.Flags().StringVar(&createFlags.acceptance, "acceptance", "", "Acceptance criteria")
	createCmd.Flags().StringVarP(&createFlags.ticketType, "type", "t", "task", "Type (bug|feature|story|investigation|task|epic|chore)")
	createCmd.Flags().StringVar(&createFlags.delivery, "delivery", "", "Delivery (code|documentation|evidence|none; default follows type)")
	createCmd.Flags().IntVarP(&createFlags.priority, "priority", "p", tk.DefaultPriority, fmt.Sprintf("Priority %d-%d, %d=highest", tk.MinPriority, tk.MaxPriority, tk.MinPriority))
	createCmd.Flags().StringVarP(&createFlags.assignee, "assignee", "a", "", "Assignee")
	createCmd.Flags().StringVar(&createFlags.externalRef, "external-ref", "", "External reference (e.g., gh-123, JIRA-456)")
	createCmd.Flags().StringVar(&createFlags.pr, "pr", "", "Pull/merge request reference (e.g., gh-pr-42, !123, URL)")
	createCmd.Flags().StringVar(&createFlags.parent, "parent", "", "Parent ticket ID")
	createCmd.Flags().StringSliceVar(&createFlags.tags, "tags", nil, "Comma-separated tags")
}
