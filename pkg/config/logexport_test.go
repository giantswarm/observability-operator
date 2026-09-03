package config

import (
	"strings"
	"testing"
)

func TestLogExportConfigValidate(t *testing.T) {
	valid := LogExportConfig{Replicas: 2, WALSize: "10Gi", ExportTimeout: "5m"}

	tests := []struct {
		name string
		// mutate turns the valid config into the one under test. wantErr is the fragment
		// the message must contain; empty means the config is accepted.
		mutate  func(*LogExportConfig)
		wantErr string
	}{
		{
			name:   "defaults",
			mutate: func(*LogExportConfig) {},
		},
		{
			name:    "zero replicas",
			mutate:  func(c *LogExportConfig) { c.Replicas = 0 },
			wantErr: "replicas must be at least 1",
		},
		{
			name:    "negative replicas",
			mutate:  func(c *LogExportConfig) { c.Replicas = -1 },
			wantErr: "replicas must be at least 1",
		},
		{
			name:    "no WAL size",
			mutate:  func(c *LogExportConfig) { c.WALSize = "" },
			wantErr: "WAL size must be set",
		},
		{
			name:    "no export timeout",
			mutate:  func(c *LogExportConfig) { c.ExportTimeout = "" },
			wantErr: "export timeout must be set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := valid
			tt.mutate(&c)

			err := c.Validate()
			switch {
			case tt.wantErr == "" && err != nil:
				t.Errorf("Validate() failed: %v", err)
			case tt.wantErr != "" && err == nil:
				t.Errorf("Validate() succeeded, expected an error about %q", tt.wantErr)
			case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
				t.Errorf("Validate() error does not mention %q: %v", tt.wantErr, err)
			}
		})
	}
}
