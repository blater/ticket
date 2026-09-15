package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ConfigSuite struct {
	suite.Suite
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}

func (s *ConfigSuite) TestLoadWithEnvVar() {
	customDir := "/custom/tickets/dir"
	s.T().Setenv(EnvTicketsDir, customDir)

	cfg, err := Load()

	require.NoError(s.T(), err)
	require.NotNil(s.T(), cfg)
	require.Equal(s.T(), "hex", cfg.Strategy)
	require.Equal(s.T(), customDir, cfg.TicketsDir)
}

func (s *ConfigSuite) TestLoadWithDefaultDir() {
	s.T().Setenv(EnvTicketsDir, "")
	tempDir := s.T().TempDir()
	s.changeWorkingDirectory(tempDir)

	cfg, err := Load()

	require.NoError(s.T(), err)
	require.NotNil(s.T(), cfg)
	require.Equal(s.T(), "hex", cfg.Strategy)

	workingDir, err := os.Getwd()
	require.NoError(s.T(), err)
	expectedDir := filepath.Join(workingDir, DefaultTicketsDir)
	require.Equal(s.T(), expectedDir, cfg.TicketsDir)
}

func (s *ConfigSuite) TestLoadWithProjectConfigFromNestedDirectory() {
	s.T().Setenv(EnvTicketsDir, "")
	projectDir := s.T().TempDir()
	nestedDir := filepath.Join(projectDir, "src", "nested")
	require.NoError(s.T(), os.MkdirAll(nestedDir, 0755))
	require.NoError(s.T(), os.WriteFile(
		filepath.Join(projectDir, ProjectConfigName),
		[]byte("tickets-directory: docs/tickets\n"),
		0644,
	))
	s.changeWorkingDirectory(nestedDir)

	cfg, err := Load()

	require.NoError(s.T(), err)
	resolvedProjectDir, err := filepath.EvalSymlinks(projectDir)
	require.NoError(s.T(), err)
	require.Equal(s.T(), filepath.Join(resolvedProjectDir, "docs", "tickets"), cfg.TicketsDir)
	require.Equal(s.T(), filepath.Join(resolvedProjectDir, ProjectConfigName), cfg.ConfigPath)
	require.Equal(s.T(), "hex", cfg.Strategy)
}

func (s *ConfigSuite) TestEnvironmentOverridesProjectConfig() {
	projectDir := s.T().TempDir()
	require.NoError(s.T(), os.WriteFile(
		filepath.Join(projectDir, ProjectConfigName),
		[]byte("tickets-directory: docs/tickets\n"),
		0644,
	))
	s.changeWorkingDirectory(projectDir)
	s.T().Setenv(EnvTicketsDir, "/override/tickets")

	cfg, err := Load()

	require.NoError(s.T(), err)
	require.Equal(s.T(), "/override/tickets", cfg.TicketsDir)
	resolvedProjectDir, err := filepath.EvalSymlinks(projectDir)
	require.NoError(s.T(), err)
	require.Equal(s.T(), filepath.Join(resolvedProjectDir, ProjectConfigName), cfg.ConfigPath)
}

func (s *ConfigSuite) TestProjectConfigLoadsDeliveryPolicy() {
	s.T().Setenv(EnvTicketsDir, "")
	projectDir := s.T().TempDir()
	require.NoError(s.T(), os.WriteFile(
		filepath.Join(projectDir, ProjectConfigName),
		[]byte("tickets-directory: docs/tickets\nstrategy: tolkien\ndelivery:\n  require-commit-links: true\n  commit-trailer: Work-Item\n  require-ticket-branch: true\n  branch-prefix: ticket/\n"),
		0644,
	))
	s.changeWorkingDirectory(projectDir)

	cfg, err := Load()

	require.NoError(s.T(), err)
	require.Equal(s.T(), "tolkien", cfg.Strategy)
	require.True(s.T(), cfg.Delivery.RequireCommitLinks)
	require.Equal(s.T(), "Work-Item", cfg.Delivery.CommitTrailer)
	require.True(s.T(), cfg.Delivery.RequireTicketBranch)
	require.Equal(s.T(), "ticket/", cfg.Delivery.BranchPrefix)
}

func (s *ConfigSuite) TestProjectConfigRejectsUnknownStrategy() {
	s.T().Setenv(EnvTicketsDir, "")
	projectDir := s.T().TempDir()
	require.NoError(s.T(), os.WriteFile(
		filepath.Join(projectDir, ProjectConfigName),
		[]byte("tickets-directory: docs/tickets\nstrategy: random\n"),
		0644,
	))
	s.changeWorkingDirectory(projectDir)

	_, err := Load()

	require.ErrorContains(s.T(), err, "valid values: default|tolkien|hex|base32|ulid")
}

func (s *ConfigSuite) TestProjectConfigDefaultSelectsGonameDefaultStrategy() {
	s.T().Setenv(EnvTicketsDir, "")
	projectDir := s.T().TempDir()
	require.NoError(s.T(), os.WriteFile(
		filepath.Join(projectDir, ProjectConfigName),
		[]byte("tickets-directory: docs/tickets\nstrategy: default\n"),
		0644,
	))
	s.changeWorkingDirectory(projectDir)

	cfg, err := Load()

	require.NoError(s.T(), err)
	require.Equal(s.T(), "default", cfg.Strategy)
}

func (s *ConfigSuite) TestProjectConfigRequiresBranchPrefixWhenEnforced() {
	s.T().Setenv(EnvTicketsDir, "")
	projectDir := s.T().TempDir()
	require.NoError(s.T(), os.WriteFile(
		filepath.Join(projectDir, ProjectConfigName),
		[]byte("tickets-directory: docs/tickets\ndelivery:\n  require-ticket-branch: true\n"),
		0644,
	))
	s.changeWorkingDirectory(projectDir)

	_, err := Load()

	require.ErrorContains(s.T(), err, "requires delivery.branch-prefix")
}

func (s *ConfigSuite) TestLoadWithLegacyDirectoryFromNestedDirectory() {
	s.T().Setenv(EnvTicketsDir, "")
	projectDir := s.T().TempDir()
	nestedDir := filepath.Join(projectDir, "src", "nested")
	require.NoError(s.T(), os.MkdirAll(nestedDir, 0755))
	require.NoError(s.T(), os.Mkdir(filepath.Join(projectDir, DefaultTicketsDir), 0755))
	s.changeWorkingDirectory(nestedDir)

	cfg, err := Load()

	require.NoError(s.T(), err)
	resolvedProjectDir, err := filepath.EvalSymlinks(projectDir)
	require.NoError(s.T(), err)
	require.Equal(s.T(), filepath.Join(resolvedProjectDir, DefaultTicketsDir), cfg.TicketsDir)
}

func (s *ConfigSuite) TestProjectConfigRequiresTicketsDirectory() {
	s.T().Setenv(EnvTicketsDir, "")
	projectDir := s.T().TempDir()
	require.NoError(s.T(), os.WriteFile(
		filepath.Join(projectDir, ProjectConfigName),
		[]byte("tickets-directory: \"\"\n"),
		0644,
	))
	s.changeWorkingDirectory(projectDir)

	_, err := Load()

	require.ErrorContains(s.T(), err, "requires tickets-directory")
}

func (s *ConfigSuite) TestProjectConfigRejectsUnknownFields() {
	s.T().Setenv(EnvTicketsDir, "")
	projectDir := s.T().TempDir()
	require.NoError(s.T(), os.WriteFile(
		filepath.Join(projectDir, ProjectConfigName),
		[]byte("tickets-directory: docs/tickets\nunknown: value\n"),
		0644,
	))
	s.changeWorkingDirectory(projectDir)

	_, err := Load()

	require.ErrorContains(s.T(), err, "field unknown not found")
}

func (s *ConfigSuite) TestConstants() {
	require.Equal(s.T(), "TICKETS_DIR", EnvTicketsDir)
	require.Equal(s.T(), "ticket.yaml", ProjectConfigName)
	require.Equal(s.T(), ".tickets", DefaultTicketsDir)
}

func (s *ConfigSuite) changeWorkingDirectory(path string) {
	originalDir, err := os.Getwd()
	require.NoError(s.T(), err)
	require.NoError(s.T(), os.Chdir(path))
	s.T().Cleanup(func() {
		require.NoError(s.T(), os.Chdir(originalDir))
	})
}
