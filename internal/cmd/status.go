package cmd

import (
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status <id> <status>",
	Short: "Update ticket status",
	Long:  `Update the ticket status. Defaults: open, in_progress, closed. Override allowed statuses in ticket.yaml under status. Supports partial ID matching.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		newStatus, err := cfg.ParseStatus(args[1])
		if err != nil {
			return err
		}
		return updateTicketStatus(cmd, args[0], newStatus)
	},
}
