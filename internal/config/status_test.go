package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	tk "github.com/radutopala/ticket/pkg/ticket"
)

func TestStatusConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name, yaml string
		want       []tk.Status
		wantError  string
	}{
		{name: "defaults", want: tk.ValidStatuses},
		{name: "empty mapping", yaml: "status: {}\n", want: tk.ValidStatuses},
		{name: "additive", yaml: "status:\n  planned: true\n", want: []tk.Status{"open", "in_progress", "closed", "planned"}},
		{name: "user example", yaml: "status:\n  open: false\n  in_progress: false\n  close: false\n  planned: true\n  scheduled: true\n  active: true\n  blocked: true\n  done: true\n  closed: true\n", want: []tk.Status{"closed", "active", "blocked", "done", "planned", "scheduled"}},
		{name: "all disabled", yaml: "status:\n  open: false\n  in_progress: false\n  closed: false\n", wantError: "at least one status"},
		{name: "empty name", yaml: "status:\n  '': true\n", wantError: "status names"},
		{name: "whitespace", yaml: "status:\n  'in progress': true\n", wantError: "status names"},
		{name: "invalid value", yaml: "status:\n  planned: maybe\n", wantError: "failed to parse"},
		{name: "duplicate", yaml: "status:\n  planned: true\n  planned: false\n", wantError: "failed to parse"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ProjectConfigName)
			require.NoError(t, os.WriteFile(path, []byte("tickets-directory: tickets\n"+tc.yaml), 0644))
			cfg, err := loadProjectConfig(path)
			if tc.wantError != "" {
				require.ErrorContains(t, err, tc.wantError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, cfg.ValidStatuses())
			for _, status := range tc.want {
				parsed, err := cfg.ParseStatus(string(status))
				require.NoError(t, err)
				require.Equal(t, status, parsed)
			}
			_, err = cfg.ParseStatus("unknown")
			require.ErrorContains(t, err, "invalid status")
		})
	}
	// Loading one project's overrides must not change another project's defaults.
	custom := &Config{Status: map[string]bool{"open": false, "planned": true}}
	require.False(t, custom.StatusEnabled(tk.StatusOpen))
	require.Equal(t, tk.ValidStatuses, (&Config{}).ValidStatuses())
	require.False(t, tk.Status("planned").IsValid())
}
