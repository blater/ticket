// Package config handles configuration loading for the tk CLI.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// EnvTicketsDir is the environment variable for the tickets directory.
	EnvTicketsDir = "TICKETS_DIR"
	// ProjectConfigName is the visible, source-controlled project configuration.
	ProjectConfigName = "ticket.yaml"
	// DefaultTicketsDir is the default directory for tickets.
	DefaultTicketsDir = ".tickets"
)

// Config holds the application configuration.
type Config struct {
	TicketsDir string
	ConfigPath string
	Delivery   DeliveryPolicy
}

type projectConfig struct {
	TicketsDirectory string         `yaml:"tickets-directory"`
	Delivery         DeliveryPolicy `yaml:"delivery,omitempty"`
}

// DeliveryPolicy configures source-to-ticket closure checks.
type DeliveryPolicy struct {
	RequireCommitLinks  bool   `yaml:"require-commit-links,omitempty"`
	CommitTrailer       string `yaml:"commit-trailer,omitempty"`
	RequireTicketBranch bool   `yaml:"require-ticket-branch,omitempty"`
	BranchPrefix        string `yaml:"branch-prefix,omitempty"`
}

// Load reads configuration from the environment, a project ticket.yaml, or
// the legacy .tickets directory. Relative project paths are resolved against
// the directory containing ticket.yaml.
func Load() (*Config, error) {
	ticketsDir := os.Getenv(EnvTicketsDir)
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	configPath, err := findFileInParents(cwd, ProjectConfigName)
	if err != nil {
		return nil, err
	}
	var config *Config
	if configPath != "" {
		config, err = loadProjectConfig(configPath)
		if err != nil {
			return nil, err
		}
	} else {
		config = &Config{Delivery: defaultDeliveryPolicy()}
	}

	if ticketsDir != "" {
		config.TicketsDir = ticketsDir
		return config, nil
	}
	if config.TicketsDir != "" {
		return config, nil
	}

	legacyDir, err := findDirectoryInParents(cwd, DefaultTicketsDir)
	if err != nil {
		return nil, err
	}
	if legacyDir != "" {
		config.TicketsDir = legacyDir
		return config, nil
	}

	config.TicketsDir = filepath.Join(cwd, DefaultTicketsDir)
	return config, nil
}

func loadProjectConfig(configPath string) (*Config, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open project config %s: %w", configPath, err)
	}
	defer func() { _ = file.Close() }()

	var project projectConfig
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&project); err != nil {
		return nil, fmt.Errorf("failed to parse project config %s: %w", configPath, err)
	}

	ticketsDir := strings.TrimSpace(project.TicketsDirectory)
	if ticketsDir == "" {
		return nil, fmt.Errorf("project config %s requires tickets-directory", configPath)
	}
	if !filepath.IsAbs(ticketsDir) {
		ticketsDir = filepath.Join(filepath.Dir(configPath), ticketsDir)
	}
	delivery := project.Delivery
	delivery.CommitTrailer = strings.TrimSpace(delivery.CommitTrailer)
	if delivery.CommitTrailer == "" {
		delivery.CommitTrailer = defaultDeliveryPolicy().CommitTrailer
	}
	delivery.BranchPrefix = strings.TrimSpace(delivery.BranchPrefix)
	if delivery.RequireTicketBranch && delivery.BranchPrefix == "" {
		return nil, fmt.Errorf("project config %s requires delivery.branch-prefix when require-ticket-branch is true", configPath)
	}

	return &Config{
		TicketsDir: filepath.Clean(ticketsDir),
		ConfigPath: configPath,
		Delivery:   delivery,
	}, nil
}

func defaultDeliveryPolicy() DeliveryPolicy {
	return DeliveryPolicy{CommitTrailer: "Ticket"}
}

func findFileInParents(start, name string) (string, error) {
	dir := start
	for {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		switch {
		case err == nil:
			if !info.Mode().IsRegular() {
				return "", fmt.Errorf("project config is not a regular file: %s", path)
			}
			return path, nil
		case !os.IsNotExist(err):
			return "", fmt.Errorf("failed to inspect project config %s: %w", path, err)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}

func findDirectoryInParents(start, name string) (string, error) {
	dir := start
	for {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		switch {
		case err == nil:
			if !info.IsDir() {
				return "", fmt.Errorf("ticket path is not a directory: %s", path)
			}
			return path, nil
		case !os.IsNotExist(err):
			return "", fmt.Errorf("failed to inspect ticket directory %s: %w", path, err)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}
