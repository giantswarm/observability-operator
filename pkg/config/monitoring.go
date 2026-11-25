package config

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	"github.com/giantswarm/observability-operator/pkg/monitoring/sharding"
)

const MonitoringLabel = "observability.giantswarm.io/monitoring"
const NetworkMonitoringLabel = "observability.giantswarm.io/network-monitoring"
const KEDAAuthenticationLabel = "observability.giantswarm.io/keda-authentication"
const KEDANamespaceAnnotation = "observability.giantswarm.io/keda-namespace"

// QueueConfig represents the configuration for the remote write queue.
type QueueConfig struct {
	BatchSendDeadline *string
	Capacity          *int
	MaxBackoff        *string
	MaxSamplesPerSend *int
	MaxShards         *int
	MinBackoff        *string
	MinShards         *int
	RetryOnHttp429    *bool
	SampleAgeLimit    *string
}

// Labels a metrics drop rule matches on.
const (
	jobLabel         = "job"
	metricsPathLabel = "metrics_path"
	metricNameLabel  = "__name__"
)

// MetricsDropRule drops matching series in Alloy before they are remote-written to Mimir.
// Job and MetricsPath, when set, restrict the rule to series with that exact job and
// metrics_path label, so a metric name can be dropped for one exporter only.
type MetricsDropRule struct {
	// Enabled set to false turns the rule off, so a configuration layer can disable a
	// rule it inherits without restating the others. Unset means enabled.
	Enabled     *bool  `json:"enabled,omitempty"`
	Job         string `json:"job,omitempty"`
	MetricsPath string `json:"metricsPath,omitempty"`
	// Regex is matched against the metric name, fully anchored like any Prometheus relabel regex.
	Regex string `json:"regex"`
}

// IsEnabled reports whether the rule applies. A nil rule is how a configuration layer
// removes an inherited rule, so it counts as disabled.
func (r *MetricsDropRule) IsEnabled() bool {
	return r != nil && (r.Enabled == nil || *r.Enabled)
}

// SourceLabels returns the relabel source labels, in the order RelabelRegex expects.
func (r *MetricsDropRule) SourceLabels() []string {
	labels := []string{}
	if r.Job != "" {
		labels = append(labels, jobLabel)
	}
	if r.MetricsPath != "" {
		labels = append(labels, metricsPathLabel)
	}
	return append(labels, metricNameLabel)
}

// RelabelRegex returns the regex matched against SourceLabels joined with ";".
func (r *MetricsDropRule) RelabelRegex() string {
	parts := []string{}
	if r.Job != "" {
		parts = append(parts, regexp.QuoteMeta(r.Job))
	}
	if r.MetricsPath != "" {
		parts = append(parts, regexp.QuoteMeta(r.MetricsPath))
	}
	return strings.Join(append(parts, "(?:"+r.Regex+")"), ";")
}

// EnabledMetricsDropRuleNames returns the names of the enabled rules, sorted so the rendered
// Alloy configuration is stable.
func (c MonitoringConfig) EnabledMetricsDropRuleNames() []string {
	names := []string{}
	for name, rule := range c.MetricsDrop {
		if rule.IsEnabled() {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// MonitoringConfig represents the configuration used by the monitoring package.
type MonitoringConfig struct {
	// Enabled controls monitoring at the installation level
	Enabled bool

	// NetworkEnabled controls network monitoring at the installation level
	NetworkEnabled bool

	AlertmanagerURL string

	DefaultShardingStrategy sharding.Strategy
	// WALTruncateFrequency is the frequency at which the WAL segments should be truncated.
	WALTruncateFrequency time.Duration
	MetricsQueryURL      string
	// RulerURL is the URL to the Mimir ruler API used to clean up rules on cluster deletion.
	// Leave empty to disable ruler cleanup.
	RulerURL    string
	QueueConfig QueueConfig
	// ExemplarsEnabled controls whether exemplars are forwarded in the remote write pipeline.
	// Uses opt-out model: enabled by default.
	ExemplarsEnabled bool

	// VCenterEnabled controls whether vCenter resource metrics are collected on vSphere and
	// Cloud Director installations. Uses opt-in model: disabled by default.
	VCenterEnabled bool

	// Gateway holds the namespace and secret names for the Mimir gateway authentication secrets.
	Gateway GatewayConfig

	// MimirRemoteWriteTimeout is the remote_timeout for the Mimir remote write endpoint in Alloy
	// agent ConfigMaps (e.g. "60s").
	MimirRemoteWriteTimeout string

	// MetricsDrop holds named rules for series Alloy drops before remote write. Rules are keyed
	// by name so each configuration layer can add, replace or disable one of them.
	MetricsDrop map[string]*MetricsDropRule
}

// Validate validates the monitoring configuration
func (c MonitoringConfig) Validate() error {
	for _, name := range c.EnabledMetricsDropRuleNames() {
		rule := c.MetricsDrop[name]
		if rule.Regex == "" {
			return fmt.Errorf("metrics drop rule %q: regex is empty", name)
		}
		if _, err := regexp.Compile("^(?:" + rule.RelabelRegex() + ")$"); err != nil {
			return fmt.Errorf("metrics drop rule %q: invalid regex: %w", name, err)
		}
	}
	return nil
}

// IsMonitoringEnabled checks if monitoring is enabled for a cluster.
// Uses opt-out model: enabled by default unless explicitly disabled.
func (c MonitoringConfig) IsMonitoringEnabled(cluster *clusterv1.Cluster) bool {
	return isClusterFeatureEnabled(c.Enabled, cluster, MonitoringLabel, true)
}

// IsNetworkMonitoringEnabled checks if network monitoring is enabled for a cluster.
// Uses opt-in model: disabled by default, must be explicitly enabled.
// TODO revisit this logic in the future when network monitoring is more widely adopted.
func (c MonitoringConfig) IsNetworkMonitoringEnabled(cluster *clusterv1.Cluster) bool {
	return isClusterFeatureEnabled(c.NetworkEnabled, cluster, NetworkMonitoringLabel, false)
}

const KEDADefaultNamespace = "keda"

// GetKEDANamespace returns the KEDA operator namespace configured for a cluster via annotation.
// Defaults to "keda" if the annotation is not set.
func GetKEDANamespace(cluster *clusterv1.Cluster) string {
	annotations := cluster.GetAnnotations()
	if annotations != nil {
		if ns, ok := annotations[KEDANamespaceAnnotation]; ok && ns != "" {
			return ns
		}
	}
	return KEDADefaultNamespace
}

// IsKEDAAuthenticationEnabled checks if KEDA authentication is enabled for a cluster.
// Uses opt-in model: disabled by default, must be explicitly enabled.
// When enabled, creates a ClusterTriggerAuthentication resource for KEDA ScaledObjects
// to authenticate with Mimir for querying metrics.
func (c MonitoringConfig) IsKEDAAuthenticationEnabled(cluster *clusterv1.Cluster) bool {
	return isClusterFeatureEnabled(c.Enabled, cluster, KEDAAuthenticationLabel, false)
}
