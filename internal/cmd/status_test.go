package cmd

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/stretchr/testify/require"

	tk "github.com/radutopala/ticket/pkg/ticket"
)

func (s *CmdSuite) TestConfiguredStatuses() {
	changeDirectory(s.T(), s.tempDir)
	path := filepath.Join(s.tempDir, "ticket.yaml")
	require.NoError(s.T(), os.WriteFile(path, []byte("tickets-directory: .\nstatus:\n  open: false\n  in_progress: false\n  closed: false\n  planned: true\n  active: true\n"), 0644))
	s.createTestTicket("tic-configured", tk.StatusOpen, "Configured status")
	_, err := s.executeCommand("status", "tic-configured", "planned")
	require.NoError(s.T(), err)
	ticket, err := store.Read("tic-configured")
	require.NoError(s.T(), err)
	require.Equal(s.T(), tk.Status("planned"), ticket.Status)
	require.Empty(s.T(), validateTickets([]*tk.Ticket{ticket}))
	for _, args := range [][]string{
		{"status", ticket.ID, "open"}, {"status", ticket.ID, "unknown"},
		{"start", ticket.ID}, {"reopen", ticket.ID}, {"close", ticket.ID},
		{"bulk", "start"}, {"bulk", "reopen"}, {"bulk", "close"},
		{"create", "Needs explicit status"},
	} {
		_, err := s.executeCommand(args...)
		require.ErrorContains(s.T(), err, "invalid status")
		unchanged, err := store.Read(ticket.ID)
		require.NoError(s.T(), err)
		require.Equal(s.T(), ticket.Status, unchanged.Status)
	}
	output, err := s.executeCommand("create", "Custom creation", "--status", "active")
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), output)
	tickets, err := store.List()
	require.NoError(s.T(), err)
	require.Len(s.T(), tickets, 2)
	output, err = s.executeCommand("list", "--status", "planned")
	require.NoError(s.T(), err)
	require.Contains(s.T(), output, "tic-configured")
	require.NotContains(s.T(), output, "Custom creation")
	imported, err := convertImportTicket(importTicket{Status: "active"})
	require.NoError(s.T(), err)
	require.Equal(s.T(), tk.Status("active"), imported.Status)
	_, err = convertImportTicket(importTicket{Status: "open"})
	require.Error(s.T(), err)
	_, err = convertImportTicket(importTicket{})
	require.Error(s.T(), err)
	var stats bytes.Buffer
	require.NoError(s.T(), outputStatsText(&stats, tk.ComputeStats(tickets)))
	require.Contains(s.T(), stats.String(), "planned:")
	require.Contains(s.T(), stats.String(), "active:")
	// Removing configuration restores defaults even within the same process.
	require.NoError(s.T(), os.Remove(path))
	_, err = s.executeCommand("status", ticket.ID, "open")
	require.NoError(s.T(), err)
	_, err = s.executeCommand("status", ticket.ID, "planned")
	require.Error(s.T(), err)
}

func (s *CmdSuite) TestCreateStatusRespectsDeliveryPolicy() {
	changeDirectory(s.T(), s.tempDir)
	require.NoError(s.T(), os.WriteFile(filepath.Join(s.tempDir, "ticket.yaml"), []byte("tickets-directory: .\ndelivery:\n  require-commit-links: true\n  require-ticket-branch: true\n  branch-prefix: ticket/\n"), 0644))
	_, err := s.executeCommand("create", "Cannot skip closure checks", "--status", "closed")
	require.ErrorContains(s.T(), err, "delivery policy requires tk close")
	_, err = s.executeCommand("create", "Cannot skip branch checks", "--status", "in_progress")
	require.ErrorContains(s.T(), err, "branch policy requires tk start")
	tickets, err := store.List()
	require.NoError(s.T(), err)
	require.Empty(s.T(), tickets)
}
