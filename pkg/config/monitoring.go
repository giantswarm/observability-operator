package config

import (
	"time"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

	"github.com/giantswarm/observability-operator/pkg/monitoring/sharding"
)

// TODO rename to observability.giantswarm.io/monitoring
const MonitoringLabel = "giantswarm.io/monitoring"

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

	AlertmanagerSecretName string
	AlertmanagerURL        string
	AlertmanagerEnabled    bool

	DefaultShardingStrategy sharding.Strategy
	// WALTruncateFrequency is the frequency at which the WAL segments should be truncated.
	WALTruncateFrequency time.Duration
	MetricsQueryURL      string
	QueueConfig          QueueConfig
}

// Validate validates the monitoring configuration
func (c MonitoringConfig) Validate() error {
	// Add validation logic here if needed
	// For now, monitoring config is always valid
	return nil
}

// Monitoring is enabled when all conditions are met:
//   - monitoring is enabled at the installation level (global flag)
//   - cluster is not being deleted
//   - cluster-specific monitoring label is set to true (or missing/invalid, defaulting to true)
func (c MonitoringConfig) IsMonitoringEnabled(cluster *clusterv1.Cluster) bool {
	return isClusterFeatureEnabled(c.Enabled, cluster, MonitoringLabel)
}
