package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/radutopala/ticket/internal/config"
	tk "github.com/radutopala/ticket/pkg/ticket"
)

func TestValidateTicketsEnforcesConfiguredDeliveryLinks(t *testing.T) {
	repositoryDir, commit := initializeDeliveryRepository(t, "Deliver story\n\nTicket: river-code\n")
	changeDirectory(t, repositoryDir)
	previousConfig := cfg
	cfg = &config.Config{Delivery: config.DeliveryPolicy{
		RequireCommitLinks: true,
		CommitTrailer:      "Ticket",
	}}
	t.Cleanup(func() { cfg = previousConfig })

	tickets := []*tk.Ticket{
		{
			ID:              "river-code",
			Status:          tk.StatusClosed,
			Type:            tk.TypeStory,
			Priority:        1,
			Delivery:        tk.DeliveryCode,
			DeliveredCommit: commit,
		},
		{
			ID:       "river-evidence",
			Status:   tk.StatusClosed,
			Type:     tk.TypeInvestigation,
			Priority: 1,
			Delivery: tk.DeliveryEvidence,
			Evidence: []string{"report://run-1"},
		},
		{
			ID:       "river-epic",
			Status:   tk.StatusClosed,
			Type:     tk.TypeEpic,
			Priority: 1,
			Delivery: tk.DeliveryNone,
		},
	}

	require.Empty(t, validateTickets(tickets))

	tickets[0].DeliveredCommit = ""
	issues := validateTickets(tickets)
	require.Len(t, issues, 1)
	require.Equal(t, "invalid-delivery", issues[0].Code)
	require.Contains(t, issues[0].Message, "requires delivered-commit")
}

func TestValidateTicketsRejectsWrongTicketTrailer(t *testing.T) {
	repositoryDir, commit := initializeDeliveryRepository(t, "Deliver story\n\nTicket: river-other\n")
	changeDirectory(t, repositoryDir)
	previousConfig := cfg
	cfg = &config.Config{Delivery: config.DeliveryPolicy{
		RequireCommitLinks: true,
		CommitTrailer:      "Ticket",
	}}
	t.Cleanup(func() { cfg = previousConfig })

	issues := validateTickets([]*tk.Ticket{{
		ID:              "river-code",
		Status:          tk.StatusClosed,
		Type:            tk.TypeStory,
		Priority:        1,
		Delivery:        tk.DeliveryCode,
		DeliveredCommit: commit,
	}})

	require.Len(t, issues, 1)
	require.Contains(t, issues[0].Message, "lacks Ticket: river-code trailer")
}

func TestValidateTicketsEnforcesRecordedTicketBranch(t *testing.T) {
	previousConfig := cfg
	cfg = &config.Config{Delivery: config.DeliveryPolicy{
		CommitTrailer:       "Ticket",
		RequireTicketBranch: true,
		BranchPrefix:        "ticket/",
	}}
	t.Cleanup(func() { cfg = previousConfig })

	ticket := &tk.Ticket{
		ID:       "river-branch",
		Status:   tk.StatusInProgress,
		Type:     tk.TypeStory,
		Priority: 1,
		Delivery: tk.DeliveryCode,
		Branch:   "feature/unlinked",
	}
	issues := validateTickets([]*tk.Ticket{ticket})
	require.Len(t, issues, 1)
	require.Contains(t, issues[0].Message, "does not use ticket/river-branch")

	ticket.Branch = "ticket/river-branch-description"
	require.Empty(t, validateTickets([]*tk.Ticket{ticket}))
}

func initializeDeliveryRepository(t *testing.T, message string) (string, string) {
	t.Helper()
	repositoryDir := t.TempDir()
	runDeliveryGit(t, repositoryDir, "init", "-b", "main")
	runDeliveryGit(t, repositoryDir, "config", "user.name", "Ticket Test")
	runDeliveryGit(t, repositoryDir, "config", "user.email", "ticket@example.invalid")
	require.NoError(t, os.WriteFile(filepath.Join(repositoryDir, "file.txt"), []byte("content\n"), 0644))
	runDeliveryGit(t, repositoryDir, "add", "file.txt")
	runDeliveryGit(t, repositoryDir, "commit", "-m", message)
	commit := runDeliveryGit(t, repositoryDir, "rev-parse", "HEAD")
	return repositoryDir, commit
}

func runDeliveryGit(t *testing.T, repositoryDir string, args ...string) string {
	t.Helper()
	commandArgs := append([]string{"-C", repositoryDir}, args...)
	output, err := exec.Command("git", commandArgs...).CombinedOutput()
	require.NoError(t, err, string(output))
	return strings.TrimSpace(string(output))
}

func changeDirectory(t *testing.T, path string) {
	t.Helper()
	original, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(path))
	t.Cleanup(func() { require.NoError(t, os.Chdir(original)) })
}
