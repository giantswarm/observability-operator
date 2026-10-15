package config

import (
	"math"
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

const (
	errHTTPTimeoutRange     = "HTTP timeout must be whole seconds from 1s to 999999s"
	errProbeTimeoutRange    = "probe timeout must be whole seconds from 1s to 999999s"
	errHealthAgeMarginRange = "health age margin must be whole seconds from 1s to 999999s"
	errProbeTimeoutMargin   = "must be at least 2s lower than timeoutSeconds"
)

func TestMimirRulesLivenessProbeConfigValidate(t *testing.T) {
	tests := []struct {
		name string
		// mutate turns the default config into the one under test. wantErr is the fragment
		// the message must contain; empty means the config is accepted.
		mutate  func(*MimirRulesLivenessProbeConfig)
		wantErr string
	}{
		{
			name:   "defaults",
			mutate: func(*MimirRulesLivenessProbeConfig) {},
		},
		{
			name: "disabled with invalid values",
			mutate: func(c *MimirRulesLivenessProbeConfig) {
				c.Enabled = false
				c.ProbeTimeout = 2 * time.Minute
				c.UnhealthyMessagePattern = "("
			},
		},
		{
			name: "smallest values",
			mutate: func(c *MimirRulesLivenessProbeConfig) {
				c.HTTPTimeout = time.Second
				c.ProbeTimeout = time.Second
				c.HealthAgeMargin = time.Second
				c.InitialDelaySeconds = 0
				c.PeriodSeconds = 1
				c.TimeoutSeconds = 3
				c.FailureThreshold = 1
			},
		},
		{
			name: "largest values",
			mutate: func(c *MimirRulesLivenessProbeConfig) {
				c.HTTPTimeout = 999999 * time.Second
				c.ProbeTimeout = 999999 * time.Second
				c.HealthAgeMargin = 999999 * time.Second
				c.InitialDelaySeconds = math.MaxInt32
				c.PeriodSeconds = math.MaxInt32
				c.TimeoutSeconds = math.MaxInt32
				c.FailureThreshold = math.MaxInt32
			},
		},
		{
			name:   "HTTP timeout equal to probe timeout",
			mutate: func(c *MimirRulesLivenessProbeConfig) { c.HTTPTimeout = c.ProbeTimeout },
		},
		{
			name:    "HTTP timeout below 1s",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.HTTPTimeout = 500 * time.Millisecond },
			wantErr: errHTTPTimeoutRange,
		},
		{
			name:    "HTTP timeout not whole seconds",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.HTTPTimeout = 1500 * time.Millisecond },
			wantErr: errHTTPTimeoutRange,
		},
		{
			name:    "HTTP timeout above 999999s",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.HTTPTimeout = 1000000 * time.Second },
			wantErr: errHTTPTimeoutRange,
		},
		{
			name:    "probe timeout below 1s",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.ProbeTimeout = 0 },
			wantErr: errProbeTimeoutRange,
		},
		{
			name:    "probe timeout not whole seconds",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.ProbeTimeout = 50*time.Second + time.Millisecond },
			wantErr: errProbeTimeoutRange,
		},
		{
			name:    "probe timeout above 999999s",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.ProbeTimeout = 1000000 * time.Second },
			wantErr: errProbeTimeoutRange,
		},
		{
			name:    "health age margin below 1s",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.HealthAgeMargin = 0 },
			wantErr: errHealthAgeMarginRange,
		},
		{
			name:    "health age margin not whole seconds",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.HealthAgeMargin = 90500 * time.Millisecond },
			wantErr: errHealthAgeMarginRange,
		},
		{
			name:    "health age margin above 999999s",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.HealthAgeMargin = 1000000 * time.Second },
			wantErr: errHealthAgeMarginRange,
		},
		{
			name:    "HTTP timeout above probe timeout",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.HTTPTimeout = 55 * time.Second },
			wantErr: "must not exceed probe timeout",
		},
		{
			name:    "empty unhealthy message pattern",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.UnhealthyMessagePattern = "" },
			wantErr: "unhealthy message pattern must not be empty",
		},
		{
			name:    "invalid unhealthy message pattern",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.UnhealthyMessagePattern = "^(error GET" },
			wantErr: "unhealthy message pattern \"^(error GET\" is invalid",
		},
		{
			name:    "relative Mimir ready path",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.MimirReadyPath = "prometheus/config/v1/rules" },
			wantErr: "ready path must start with /",
		},
		{
			name:    "empty Mimir ready path",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.MimirReadyPath = "" },
			wantErr: "ready path must start with /",
		},
		{
			name:    "negative initialDelaySeconds",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.InitialDelaySeconds = -1 },
			wantErr: "initialDelaySeconds must be from 0 to 2147483647",
		},
		{
			name:    "initialDelaySeconds above int32",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.InitialDelaySeconds = math.MaxInt32 + 1 },
			wantErr: "initialDelaySeconds must be from 0 to 2147483647",
		},
		{
			name:    "periodSeconds below 1",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.PeriodSeconds = 0 },
			wantErr: "periodSeconds must be from 1 to 2147483647",
		},
		{
			name:    "periodSeconds above int32",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.PeriodSeconds = math.MaxInt32 + 1 },
			wantErr: "periodSeconds must be from 1 to 2147483647",
		},
		{
			name:    "timeoutSeconds below 1",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.TimeoutSeconds = 0 },
			wantErr: "timeoutSeconds must be from 1 to 2147483647",
		},
		{
			name:    "timeoutSeconds above int32",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.TimeoutSeconds = math.MaxInt32 + 1 },
			wantErr: "timeoutSeconds must be from 1 to 2147483647",
		},
		{
			name:    "failureThreshold below 1",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.FailureThreshold = 0 },
			wantErr: "failureThreshold must be from 1 to 2147483647",
		},
		{
			name:    "failureThreshold above int32",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.FailureThreshold = math.MaxInt32 + 1 },
			wantErr: "failureThreshold must be from 1 to 2147483647",
		},
		{
			name:   "probe timeout 2s below timeoutSeconds",
			mutate: func(c *MimirRulesLivenessProbeConfig) { c.ProbeTimeout = 58 * time.Second },
		},
		{
			name:    "probe timeout 1s below timeoutSeconds",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.ProbeTimeout = 59 * time.Second },
			wantErr: errProbeTimeoutMargin,
		},
		{
			name:    "probe timeout equal to timeoutSeconds",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.ProbeTimeout = 60 * time.Second },
			wantErr: errProbeTimeoutMargin,
		},
		{
			name:    "probe timeout above timeoutSeconds",
			mutate:  func(c *MimirRulesLivenessProbeConfig) { c.TimeoutSeconds = 30 },
			wantErr: errProbeTimeoutMargin,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := DefaultMimirRulesLivenessProbeConfig()
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

// TestMonitoringConfigValidate asserts the startup validation covers the liveness probe.
func TestMonitoringConfigValidate(t *testing.T) {
	c := MonitoringConfig{MimirRulesLivenessProbe: DefaultMimirRulesLivenessProbeConfig()}
	if err := c.Validate(); err != nil {
		t.Errorf("Validate() failed: %v", err)
	}
	c.MimirRulesLivenessProbe.HTTPTimeout = 1500 * time.Millisecond
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "mimir rules liveness probe: HTTP timeout") {
		t.Errorf("Validate() error = %v, expected a mimir rules liveness probe error", err)
	}
}
