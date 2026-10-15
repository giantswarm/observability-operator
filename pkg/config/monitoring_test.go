package config

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"sigs.k8s.io/yaml"
)

// mimirRulesLivenessProbeScriptDefaults are the defaults of the probe script
// helm/alloy/scripts/mimir-rules-liveness-probe.sh in giantswarm/alloy-app at SCRIPT_COMMIT.
// Update them, and docs/cluster.md, when the script defaults change.
var mimirRulesLivenessProbeScriptDefaults = map[string]string{
	"HTTP_TIMEOUT":              "20",
	"PROBE_TIMEOUT":             "50",
	"HEALTH_AGE_MARGIN":         "60",
	"UNHEALTHY_MESSAGE_PATTERN": `^(error GET [^ ]*/rules: server returned HTTP status (429|5[0-9]{2}) |Get "[^"]*/rules": )`,
	"MIMIR_READY_PATH":          "/prometheus/config/v1/rules",
}

// TestDefaultMimirRulesLivenessProbeConfigMatchesScript asserts the operator renders the
// probe script defaults. The script is the single source of the defaults.
func TestDefaultMimirRulesLivenessProbeConfigMatchesScript(t *testing.T) {
	c := DefaultMimirRulesLivenessProbeConfig()
	got := map[string]string{
		"HTTP_TIMEOUT":              strconv.Itoa(int(c.HTTPTimeout.Seconds())),
		"PROBE_TIMEOUT":             strconv.Itoa(int(c.ProbeTimeout.Seconds())),
		"HEALTH_AGE_MARGIN":         strconv.Itoa(int(c.HealthAgeMargin.Seconds())),
		"UNHEALTHY_MESSAGE_PATTERN": c.UnhealthyMessagePattern,
		"MIMIR_READY_PATH":          c.MimirReadyPath,
	}
	if diff := cmp.Diff(mimirRulesLivenessProbeScriptDefaults, got); diff != "" {
		t.Errorf("operator defaults differ from the probe script defaults (-script +operator):\n%s", diff)
	}
}

// TestDefaultMimirRulesLivenessProbeConfigMatchesHelmValues asserts the Helm chart values
// default to the operator defaults.
func TestDefaultMimirRulesLivenessProbeConfigMatchesHelmValues(t *testing.T) {
	data, err := os.ReadFile("../../helm/observability-operator/values.yaml")
	if err != nil {
		t.Fatalf("failed to read Helm values: %v", err)
	}
	var values struct {
		Monitoring struct {
			MimirRulesLivenessProbe struct {
				Enabled                 bool   `json:"enabled"`
				HTTPTimeout             string `json:"httpTimeout"`
				ProbeTimeout            string `json:"probeTimeout"`
				HealthAgeMargin         string `json:"healthAgeMargin"`
				UnhealthyMessagePattern string `json:"unhealthyMessagePattern"`
				MimirReadyPath          string `json:"mimirReadyPath"`
				InitialDelaySeconds     int    `json:"initialDelaySeconds"`
				PeriodSeconds           int    `json:"periodSeconds"`
				TimeoutSeconds          int    `json:"timeoutSeconds"`
				FailureThreshold        int    `json:"failureThreshold"`
			} `json:"mimirRulesLivenessProbe"`
		} `json:"monitoring"`
	}
	if err := yaml.Unmarshal(data, &values); err != nil {
		t.Fatalf("failed to unmarshal Helm values: %v", err)
	}
	v := values.Monitoring.MimirRulesLivenessProbe

	got := MimirRulesLivenessProbeConfig{
		Enabled:                 v.Enabled,
		UnhealthyMessagePattern: v.UnhealthyMessagePattern,
		MimirReadyPath:          v.MimirReadyPath,
		InitialDelaySeconds:     v.InitialDelaySeconds,
		PeriodSeconds:           v.PeriodSeconds,
		TimeoutSeconds:          v.TimeoutSeconds,
		FailureThreshold:        v.FailureThreshold,
	}
	for _, d := range []struct {
		value string
		into  *time.Duration
	}{
		{v.HTTPTimeout, &got.HTTPTimeout},
		{v.ProbeTimeout, &got.ProbeTimeout},
		{v.HealthAgeMargin, &got.HealthAgeMargin},
	} {
		if *d.into, err = time.ParseDuration(d.value); err != nil {
			t.Fatalf("failed to parse Helm value %q: %v", d.value, err)
		}
	}
	if diff := cmp.Diff(DefaultMimirRulesLivenessProbeConfig(), got); diff != "" {
		t.Errorf("Helm values differ from the operator defaults (-operator +helm):\n%s", diff)
	}
}

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
