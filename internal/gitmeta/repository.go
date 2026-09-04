// Package gitmeta owns the mapping between tickets and Git delivery identity.
package gitmeta

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var trailerKeyPattern = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

// Repository provides read-only Git identity and commit metadata operations.
type Repository struct {
	workDir string
}

// TrailerReference identifies one commit trailer value.
type TrailerReference struct {
	Commit string
	Value  string
}

// CommitTrailers contains one commit's parents and selected trailer values.
type CommitTrailers struct {
	Commit  string
	Parents []string
	Values  []string
}

// New opens Git metadata relative to workDir.
func New(workDir string) Repository {
	return Repository{workDir: workDir}
}

// CurrentContext returns the full HEAD commit and current branch name.
func (r Repository) CurrentContext() (string, string, error) {
	commit, err := r.ResolveCommit("HEAD")
	if err != nil {
		return "", "", err
	}

	branch, err := r.run("branch", "--show-current")
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve current Git branch: %w", err)
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return "", "", fmt.Errorf("cannot claim a ticket from detached HEAD")
	}

	return commit, branch, nil
}

// ResolveCommit resolves ref to a full commit object ID.
func (r Repository) ResolveCommit(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("commit reference is empty")
	}

	commit, err := r.run("rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("invalid Git commit %q: %w", ref, err)
	}
	return strings.TrimSpace(commit), nil
}

// CommitHasTrailer reports whether commit has an exact key/value trailer.
func (r Repository) CommitHasTrailer(commit, key, value string) (bool, error) {
	if !trailerKeyPattern.MatchString(key) {
		return false, fmt.Errorf("invalid Git trailer key %q", key)
	}

	format := fmt.Sprintf("%%(trailers:key=%s,valueonly)", key)
	trailers, err := r.run("show", "-s", "--format="+format, commit)
	if err != nil {
		return false, fmt.Errorf("failed to read Git trailers from %s: %w", commit, err)
	}
	for _, trailerValue := range strings.Split(trailers, "\n") {
		if strings.TrimSpace(trailerValue) == value {
			return true, nil
		}
	}
	return false, nil
}

// TrailerReferences returns all values for key in commits selected by
// revisionRange. Control separators avoid ambiguity with multiline commit
// messages and multiple trailers.
func (r Repository) TrailerReferences(revisionRange, key string) ([]TrailerReference, error) {
	commits, err := r.CommitsWithTrailers(revisionRange, key)
	if err != nil {
		return nil, err
	}

	var references []TrailerReference
	for _, commit := range commits {
		for _, value := range commit.Values {
			references = append(references, TrailerReference{Commit: commit.Commit, Value: value})
		}
	}
	return references, nil
}

// CommitsWithTrailers returns every selected commit, including commits without
// the requested trailer. Callers can therefore enforce trailer presence over
// an explicit feature revision range without scanning unrelated history.
func (r Repository) CommitsWithTrailers(revisionRange, key string) ([]CommitTrailers, error) {
	if !trailerKeyPattern.MatchString(key) {
		return nil, fmt.Errorf("invalid Git trailer key %q", key)
	}
	revisionRange = strings.TrimSpace(revisionRange)
	if revisionRange == "" || strings.HasPrefix(revisionRange, "-") {
		return nil, fmt.Errorf("invalid Git revision range %q", revisionRange)
	}

	format := fmt.Sprintf("%%H%%x1f%%P%%x1f%%(trailers:key=%s,valueonly,separator=%%x1e)%%x1d", key)
	output, err := r.run("log", "--format="+format, revisionRange)
	if err != nil {
		return nil, fmt.Errorf("failed to scan Git trailers in %q: %w", revisionRange, err)
	}

	var commits []CommitTrailers
	for _, record := range strings.Split(output, "\x1d") {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}
		parts := strings.SplitN(record, "\x1f", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("malformed Git log record for %q", revisionRange)
		}
		commit := CommitTrailers{
			Commit:  strings.TrimSpace(parts[0]),
			Parents: strings.Fields(parts[1]),
		}
		for _, value := range strings.Split(parts[2], "\x1e") {
			value = strings.TrimSpace(value)
			if value != "" {
				commit.Values = append(commit.Values, value)
			}
		}
		commits = append(commits, commit)
	}
	return commits, nil
}

func (r Repository) run(args ...string) (string, error) {
	commandArgs := args
	if r.workDir != "" {
		commandArgs = append([]string{"-C", r.workDir}, args...)
	}
	cmd := exec.Command("git", commandArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return "", err
		}
		return "", fmt.Errorf("%s: %w", message, err)
	}
	return string(output), nil
}
