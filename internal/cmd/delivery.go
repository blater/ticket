package cmd

import (
	"fmt"
	"strings"

	"github.com/radutopala/ticket/internal/gitmeta"
	tk "github.com/radutopala/ticket/pkg/ticket"
)

func validateTicketDelivery(ticket *tk.Ticket, requireClosure bool) []string {
	var issues []string
	delivery := ticket.EffectiveDelivery()
	if !delivery.IsValid() {
		return []string{fmt.Sprintf("invalid delivery classification %q", delivery)}
	}
	if delivery == tk.DeliveryNone && ticket.Type != tk.TypeEpic {
		issues = append(issues, "delivery none is valid only for epics")
	}
	if ticketRequiresBranch(ticket) {
		if ticket.Branch == "" {
			if requireClosure || ticket.Status == tk.StatusInProgress {
				issues = append(issues, "delivery requires recorded branch")
			}
		} else if !branchCarriesTicket(ticket.Branch, ticket.ID) {
			issues = append(issues, fmt.Sprintf(
				"branch %q does not use %s%s",
				ticket.Branch,
				cfg.Delivery.BranchPrefix,
				ticket.ID,
			))
		}
	}

	if requireClosure && cfg.Delivery.RequireCommitLinks {
		switch delivery {
		case tk.DeliveryCode, tk.DeliveryDocumentation:
			if ticket.DeliveredCommit == "" {
				issues = append(issues, fmt.Sprintf("%s delivery requires delivered-commit", delivery))
			}
		case tk.DeliveryEvidence:
			if ticket.DeliveredCommit == "" && len(ticket.Evidence) == 0 {
				issues = append(issues, "evidence delivery requires delivered-commit or evidence")
			}
		}
	}

	if ticket.DeliveredCommit != "" {
		repository := gitmeta.New("")
		commit, err := repository.ResolveCommit(ticket.DeliveredCommit)
		if err != nil {
			issues = append(issues, err.Error())
			return issues
		}
		ticket.DeliveredCommit = commit

		hasTrailer, err := repository.CommitHasTrailer(
			commit,
			cfg.Delivery.CommitTrailer,
			ticket.ID,
		)
		if err != nil {
			issues = append(issues, err.Error())
		} else if !hasTrailer {
			issues = append(issues, fmt.Sprintf(
				"commit %s lacks %s: %s trailer",
				commit,
				cfg.Delivery.CommitTrailer,
				ticket.ID,
			))
		}
	}

	return issues
}

func ticketRequiresBranch(ticket *tk.Ticket) bool {
	if !cfg.Delivery.RequireTicketBranch || ticket.Type == tk.TypeEpic {
		return false
	}
	delivery := ticket.EffectiveDelivery()
	return delivery == tk.DeliveryCode || delivery == tk.DeliveryDocumentation
}

func ticketRequiresCommit(ticket *tk.Ticket) bool {
	if !cfg.Delivery.RequireCommitLinks {
		return false
	}
	delivery := ticket.EffectiveDelivery()
	return delivery == tk.DeliveryCode || delivery == tk.DeliveryDocumentation
}

func branchCarriesTicket(branch, ticketID string) bool {
	required := cfg.Delivery.BranchPrefix + ticketID
	return branch == required || strings.HasPrefix(branch, required+"-") || strings.HasPrefix(branch, required+"/")
}

func appendUnique(values []string, additions ...string) []string {
	seen := make(map[string]struct{}, len(values)+len(additions))
	result := make([]string, 0, len(values)+len(additions))
	for _, value := range append(values, additions...) {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
