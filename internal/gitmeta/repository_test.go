package gitmeta

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRepositoryResolvesContextAndTrailers(t *testing.T) {
	repositoryDir := initializeRepository(t, "Implement delivery\n\nTicket: river-ab12\n")
	repository := New(repositoryDir)

	commit, branch, err := repository.CurrentContext()
	require.NoError(t, err)
	require.NotEmpty(t, commit)
	require.Equal(t, "main", branch)

	hasTrailer, err := repository.CommitHasTrailer(commit, "Ticket", "river-ab12")
	require.NoError(t, err)
	require.True(t, hasTrailer)

	hasTrailer, err = repository.CommitHasTrailer(commit, "Ticket", "river-other")
	require.NoError(t, err)
	require.False(t, hasTrailer)

	references, err := repository.TrailerReferences("HEAD", "Ticket")
	require.NoError(t, err)
	require.Equal(t, []TrailerReference{{Commit: commit, Value: "river-ab12"}}, references)

	commits, err := repository.CommitsWithTrailers("HEAD", "Ticket")
	require.NoError(t, err)
	require.Len(t, commits, 1)
	require.Equal(t, commit, commits[0].Commit)
	require.Empty(t, commits[0].Parents)
	require.Equal(t, []string{"river-ab12"}, commits[0].Values)
}

func TestRepositoryRejectsInvalidCommitAndTrailerKey(t *testing.T) {
	repositoryDir := initializeRepository(t, "Initial commit\n")
	repository := New(repositoryDir)

	_, err := repository.ResolveCommit("missing")
	require.ErrorContains(t, err, "invalid Git commit")

	_, err = repository.CommitHasTrailer("HEAD", "bad:key", "river-ab12")
	require.ErrorContains(t, err, "invalid Git trailer key")
}

func initializeRepository(t *testing.T, message string) string {
	t.Helper()
	repositoryDir := t.TempDir()
	runGit(t, repositoryDir, "init", "-b", "main")
	runGit(t, repositoryDir, "config", "user.name", "Ticket Test")
	runGit(t, repositoryDir, "config", "user.email", "ticket@example.invalid")
	require.NoError(t, os.WriteFile(filepath.Join(repositoryDir, "file.txt"), []byte("content\n"), 0644))
	runGit(t, repositoryDir, "add", "file.txt")
	runGit(t, repositoryDir, "commit", "-m", message)
	return repositoryDir
}

func runGit(t *testing.T, repositoryDir string, args ...string) {
	t.Helper()
	commandArgs := append([]string{"-C", repositoryDir}, args...)
	cmd := exec.Command("git", commandArgs...)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}
