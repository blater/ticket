package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/radutopala/ticket/internal/gitmeta"
	tk "github.com/radutopala/ticket/pkg/ticket"
)

var startCmd = &cobra.Command{
	Use:   "start <id>",
	Short: "Set ticket status to in_progress",
	Long:  `Set the ticket status to in_progress. Supports partial ID matching. Uses file locking to prevent race conditions.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := cfg.ParseStatus(string(tk.StatusInProgress)); err != nil {
			return err
		}
		id, err := store.ResolveID(args[0])
		if err != nil {
			return err
		}
		candidate, err := store.Read(id)
		if err != nil {
			return err
		}

		var prepare func(*tk.Ticket) error
		if ticketRequiresBranch(candidate) || ticketRequiresCommit(candidate) {
			baseCommit, branch, err := gitmeta.New("").CurrentContext()
			if err != nil {
				return fmt.Errorf("failed to capture claim Git context: %w", err)
			}
			if ticketRequiresBranch(candidate) && !branchCarriesTicket(branch, id) {
				return fmt.Errorf(
					"branch %q does not identify ticket %s; expected %s%s[-description]",
					branch,
					id,
					cfg.Delivery.BranchPrefix,
					id,
				)
			}
			prepare = func(ticket *tk.Ticket) error {
				ticket.BaseCommit = baseCommit
				ticket.Branch = branch
				return nil
			}
		}

		ticket, err := store.AtomicClaimWith(id, prepare)
		if err != nil {
			if errors.Is(err, tk.ErrAlreadyClaimed) {
				return fmt.Errorf("cannot claim %s: %w", id, err)
			}
			return fmt.Errorf("failed to claim ticket: %w", err)
		}

		if jsonOutput {
			return outputJSON(cmd, ticket)
		}

		fmt.Printf("Claimed %s -> in_progress\n", ticket.ID)
		return nil
	},
}
