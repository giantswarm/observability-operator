package config

import (
	"time"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

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

	// OTLPEnabled controls whether OTLP metrics ingestion is enabled via the events collector
	OTLPEnabled bool

	AlertmanagerSecretName string
	AlertmanagerURL        string
	AlertmanagerEnabled    bool

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

	// Gateway holds the namespace and secret names for the Mimir gateway authentication secrets.
	Gateway GatewayConfig

	// MimirRemoteWriteTimeout is the remote_timeout for the Mimir remote write endpoint in Alloy
	// agent ConfigMaps (e.g. "60s").
	MimirRemoteWriteTimeout string
}

// Validate validates the monitoring configuration
func (c MonitoringConfig) Validate() error {
	// Add validation logic here if needed
	// For now, monitoring config is always valid
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
