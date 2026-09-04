package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	tk "github.com/radutopala/ticket/pkg/ticket"
)

var closeFlags struct {
	commit        string
	checkpointTag string
	evidence      []string
}

var closeCmd = &cobra.Command{
	Use:   "close <id>",
	Short: "Set ticket status to closed",
	Long: `Set the ticket status to closed. When project delivery policy requires
commit links, code and documentation tickets require a delivered commit with
an exact Ticket trailer. Evidence tickets require a commit or evidence link.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ticket, err := resolveAndReadTicket(args[0])
		if err != nil {
			return err
		}

		if strings.TrimSpace(closeFlags.commit) != "" {
			ticket.DeliveredCommit = strings.TrimSpace(closeFlags.commit)
		}
		if strings.TrimSpace(closeFlags.checkpointTag) != "" {
			ticket.CheckpointTag = strings.TrimSpace(closeFlags.checkpointTag)
		}
		ticket.Evidence = appendUnique(ticket.Evidence, closeFlags.evidence...)

		if issues := validateTicketDelivery(ticket, true); len(issues) > 0 {
			return fmt.Errorf("cannot close %s: %s", ticket.ID, strings.Join(issues, "; "))
		}

		ticket.Status = tk.StatusClosed
		if err := store.Write(ticket); err != nil {
			return fmt.Errorf("failed to update ticket: %w", err)
		}

		if jsonOutput {
			return outputJSON(cmd, ticket)
		}

		fmt.Printf("Updated %s -> %s\n", ticket.ID, ticket.Status)
		return nil
	},
}

func init() {
	closeCmd.Flags().StringVar(&closeFlags.commit, "commit", "", "Delivered Git commit or ref")
	closeCmd.Flags().StringVar(&closeFlags.checkpointTag, "checkpoint-tag", "", "Stable checkpoint tag")
	closeCmd.Flags().StringSliceVar(&closeFlags.evidence, "evidence", nil, "Evidence reference (repeatable)")
}
