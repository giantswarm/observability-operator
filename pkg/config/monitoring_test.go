package config

import (
	"strings"
	"testing"
	"time"
)

func TestMimirRulesLivenessProbeConfigValidate(t *testing.T) {
	valid := MimirRulesLivenessProbeConfig{
		Enabled:             true,
		HTTPTimeout:         20 * time.Second,
		ProbeTimeout:        50 * time.Second,
		InitialDelaySeconds: 60,
		PeriodSeconds:       60,
		TimeoutSeconds:      60,
		FailureThreshold:    3,
	}

	tests := []struct {
		name string
		// mutate turns the valid config into the one under test. wantErr is the fragment
		// the message must contain; empty means the config is accepted.
		mutate  func(*MimirRulesLivenessProbeConfig)
		wantErr string
	}{
		{
			name:   "defaults",
			mutate: func(*MimirRulesLivenessProbeConfig) {},
		},
		{
			name:   "HTTP timeout equal to probe timeout",
			mutate: func(c *MimirRulesLivenessProbeConfig) { c.HTTPTimeout = c.ProbeTimeout },
		},
		{
			name: "disabled with invalid values",
			mutate: func(c *MimirRulesLivenessProbeConfig) {
				c.Enabled = false
				c.ProbeTimeout = 2 * time.Minute
			},
		},
		{
			name:    "HTTP timeout below 1s",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.HTTPTimeout = 500 * time.Millisecond },
			wantErr: "HTTP timeout must be at least 1s",
		},
		{
			name:    "HTTP timeout above probe timeout",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.HTTPTimeout = 55 * time.Second },
			wantErr: "must not exceed probe timeout",
		},
		{
			name:    "probe timeout equal to timeoutSeconds",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.ProbeTimeout = 60 * time.Second },
			wantErr: "must be lower than timeoutSeconds",
		},
		{
			name:    "probe timeout above timeoutSeconds",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.TimeoutSeconds = 30 },
			wantErr: "must be lower than timeoutSeconds",
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
