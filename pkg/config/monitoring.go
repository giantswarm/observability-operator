package config

import (
	"strconv"
	"time"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/giantswarm/observability-operator/pkg/monitoring/sharding"
)

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

// IsMonitored should be enabled when all conditions are met:
//   - global monitoring flag is enabled
//   - monitoring label is not set or is set to true on the cluster object
func (c MonitoringConfig) IsMonitored(cluster *clusterv1.Cluster) bool {
	if !c.Enabled {
		return false
	}

	// Check if label is set on the cluster object
	labels := cluster.GetLabels()
	monitoringLabelValue, ok := labels[MonitoringLabel]
	if !ok {
		// If it's not set, monitoring is enabled by default
		return true
	}

	monitoringEnabled, err := strconv.ParseBool(monitoringLabelValue)
	if err != nil {
		return true
	}
	return monitoringEnabled
}

// Validate validates the monitoring configuration
func (c MonitoringConfig) Validate() error {
	// Add validation logic here if needed
	// For now, monitoring config is always valid
	return nil
}
