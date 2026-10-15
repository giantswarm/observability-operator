package config

import (
	"fmt"
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

// MonitoringConfig represents the configuration used by the monitoring package.
type MonitoringConfig struct {
	// Enabled controls monitoring at the installation level
	Enabled bool

	// NetworkEnabled controls network monitoring at the installation level
	NetworkEnabled bool

	AlertmanagerSecretName string
	AlertmanagerURL        string

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

	// MimirRulesLivenessProbe configures the alloy-metrics liveness probe that restarts Alloy
	// when a mimir.rules.kubernetes component stays unhealthy.
	MimirRulesLivenessProbe MimirRulesLivenessProbeConfig
}

// MimirRulesLivenessProbeConfig configures the alloy-metrics mimir.rules.kubernetes liveness probe.
type MimirRulesLivenessProbeConfig struct {
	// Enabled renders the probe for observability-bundle versions supporting it.
	// Disabled, the upstream Alloy chart liveness probe applies.
	Enabled bool

	// HTTPTimeout is the timeout of each HTTP request the probe script sends.
	HTTPTimeout time.Duration
	// ProbeTimeout is the total budget of one probe script run. The script exits 0 when exceeded.
	ProbeTimeout time.Duration
	// UnhealthyMessagePattern is the extended regular expression matched against the component
	// health message.
	UnhealthyMessagePattern string
	// MimirReadyPath is the path the probe script requests on the Mimir ruler to check it answers.
	MimirReadyPath string

	InitialDelaySeconds int
	PeriodSeconds       int
	TimeoutSeconds      int
	FailureThreshold    int
}

// DefaultMimirRulesLivenessProbeConfig returns the default mimir.rules.kubernetes liveness
// probe configuration. The script settings equal the probe script defaults, see docs/cluster.md.
func DefaultMimirRulesLivenessProbeConfig() MimirRulesLivenessProbeConfig {
	return MimirRulesLivenessProbeConfig{
		Enabled:                 true,
		HTTPTimeout:             20 * time.Second,
		ProbeTimeout:            50 * time.Second,
		UnhealthyMessagePattern: `^(error GET [^ ]*/rules: server returned HTTP status (429|5[0-9]{2}) |Get "[^"]*/rules": )`,
		MimirReadyPath:          "/prometheus/config/v1/rules",
		InitialDelaySeconds:     60,
		PeriodSeconds:           60,
		TimeoutSeconds:          60,
		FailureThreshold:        3,
	}
}

// Validate validates the mimir.rules.kubernetes liveness probe configuration.
func (c MimirRulesLivenessProbeConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.HTTPTimeout < time.Second {
		return fmt.Errorf("HTTP timeout must be at least 1s, got %s", c.HTTPTimeout)
	}
	if c.HTTPTimeout > c.ProbeTimeout {
		return fmt.Errorf("HTTP timeout %s must not exceed probe timeout %s", c.HTTPTimeout, c.ProbeTimeout)
	}
	if timeout := time.Duration(c.TimeoutSeconds) * time.Second; c.ProbeTimeout >= timeout {
		return fmt.Errorf("probe timeout %s must be lower than timeoutSeconds %s", c.ProbeTimeout, timeout)
	}
	return nil
}

// Validate validates the monitoring configuration
func (c MonitoringConfig) Validate() error {
	if err := c.MimirRulesLivenessProbe.Validate(); err != nil {
		return fmt.Errorf("mimir rules liveness probe: %w", err)
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
