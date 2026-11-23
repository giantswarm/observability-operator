package config

import (
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func boolPtr(b bool) *bool { return &b }

func TestMetricsDropRuleRelabel(t *testing.T) {
	tests := []struct {
		name       string
		rule       MetricsDropRule
		wantLabels []string
		wantRegex  string
		drops      []string
		keeps      []string
	}{
		{
			name:       "name only",
			rule:       MetricsDropRule{Regex: "container_pressure_.+"},
			wantLabels: []string{metricNameLabel},
			wantRegex:  "(?:container_pressure_.+)",
			drops:      []string{"container_pressure_cpu_stalled_seconds_total"},
			keeps:      []string{"container_cpu_usage_seconds_total"},
		},
		{
			name:       "job and metrics path",
			rule:       MetricsDropRule{Job: "kubelet", MetricsPath: "/metrics", Regex: "go_(gc|memstats)_.+|go_info"},
			wantLabels: []string{jobLabel, metricsPathLabel, metricNameLabel},
			wantRegex:  "kubelet;/metrics;(?:go_(gc|memstats)_.+|go_info)",
			drops:      []string{"kubelet;/metrics;go_info", "kubelet;/metrics;go_gc_duration_seconds"},
			keeps: []string{
				"kubelet;/metrics;go_goroutines",
				"kubelet;/metrics/cadvisor;go_info",
				"mimir/ingester;/metrics;go_info",
			},
		},
		{
			name:       "job values are matched literally",
			rule:       MetricsDropRule{Job: "kube-system/cilium-agent", Regex: "cilium_hive_jobs_.+"},
			wantLabels: []string{jobLabel, metricNameLabel},
			wantRegex:  "kube-system/cilium-agent;(?:cilium_hive_jobs_.+)",
			drops:      []string{"kube-system/cilium-agent;cilium_hive_jobs_runs_total"},
			keeps:      []string{"kube-system-cilium-agent;cilium_hive_jobs_runs_total"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := cmp.Diff(tt.wantLabels, tt.rule.SourceLabels()); diff != "" {
				t.Errorf("SourceLabels() mismatch (-want +got):\n%s", diff)
			}
			if got := tt.rule.RelabelRegex(); got != tt.wantRegex {
				t.Errorf("RelabelRegex() = %q, want %q", got, tt.wantRegex)
			}
			// Prometheus and Alloy anchor relabel regexes.
			re := regexp.MustCompile("^(?:" + tt.rule.RelabelRegex() + ")$")
			for _, v := range tt.drops {
				if !re.MatchString(v) {
					t.Errorf("%q should be dropped", v)
				}
			}
			for _, v := range tt.keeps {
				if re.MatchString(v) {
					t.Errorf("%q should be kept", v)
				}
			}
		})
	}
}

func TestEnabledMetricsDropRuleNames(t *testing.T) {
	c := MonitoringConfig{MetricsDrop: map[string]*MetricsDropRule{
		"zeta":     {Regex: "z"},
		"alpha":    {Regex: "a", Enabled: boolPtr(true)},
		"disabled": {Regex: "d", Enabled: boolPtr(false)},
		"removed":  nil,
	}}
	if diff := cmp.Diff([]string{"alpha", "zeta"}, c.EnabledMetricsDropRuleNames()); diff != "" {
		t.Errorf("EnabledMetricsDropRuleNames() mismatch (-want +got):\n%s", diff)
	}
}

func TestMonitoringConfigValidateMetricsDrop(t *testing.T) {
	tests := []struct {
		name    string
		drop    map[string]*MetricsDropRule
		wantErr string
	}{
		{name: "no rules"},
		{name: "valid rule", drop: map[string]*MetricsDropRule{"ok": {Job: "apiserver", Regex: "etcd_requests_total"}}},
		{name: "disabled invalid rule is ignored", drop: map[string]*MetricsDropRule{"off": {Regex: "(", Enabled: boolPtr(false)}}},
		{name: "empty regex", drop: map[string]*MetricsDropRule{"empty": {Job: "kubelet"}}, wantErr: `metrics drop rule "empty": regex is empty`},
		{name: "invalid regex", drop: map[string]*MetricsDropRule{"broken": {Regex: "go_(gc"}}, wantErr: `metrics drop rule "broken": invalid regex`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MonitoringConfig{MetricsDrop: tt.drop}.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}
