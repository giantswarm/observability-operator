package config

import (
	"time"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

	"github.com/giantswarm/observability-operator/pkg/monitoring/sharding"
)

// TODO rename to observability.giantswarm.io/monitoring
const MonitoringLabel = "giantswarm.io/monitoring"

// TODO rename to observability.giantswarm.io/network-monitoring
const NetworkMonitoringLabel = "giantswarm.io/network-monitoring"

// TODO rename to observability.giantswarm.io/keda-authentication
const KEDAAuthenticationLabel = "giantswarm.io/keda-authentication"

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
	AlertmanagerEnabled    bool

	DefaultShardingStrategy sharding.Strategy
	// WALTruncateFrequency is the frequency at which the WAL segments should be truncated.
	WALTruncateFrequency time.Duration
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

// Network monitoring is enabled when all conditions are met:
//   - network monitoring is enabled at the installation level (global flag)
//   - cluster is not being deleted
//   - cluster-specific network monitoring label is set to "true" (defaults to false if missing/invalid)
//
// We need this special logic because network monitoring must be explicitly enabled per cluster for now.
// TODO revisit this logic in the future when network monitoring is more widely adopted.
func (c MonitoringConfig) IsNetworkMonitoringEnabled(cluster *clusterv1.Cluster) bool {
	// Check global flag
	if !c.NetworkEnabled {
		return false
	}

	// If the cluster is being deleted, always return false
	deletionTimestamp := cluster.GetDeletionTimestamp()
	if deletionTimestamp != nil && !deletionTimestamp.IsZero() {
		return false
	}

	// Check cluster-specific label - must be explicitly set to "true" (defaults to false)
	labels := cluster.GetLabels()
	if labels == nil {
		return false // default to disabled when no labels
	}

	labelValue, ok := labels[NetworkMonitoringLabel]
	if !ok {
		return false // default to disabled when label not set
	}

	// Only enabled if explicitly set to "true"
	return labelValue == "true"
}

// KEDA authentication is enabled when all conditions are met:
//   - monitoring is enabled at the installation level (global flag)
//   - cluster is not being deleted
//   - cluster-specific KEDA authentication label is set to "true" (defaults to false if missing/invalid)
//
// This creates a ClusterTriggerAuthentication resource that can be used by KEDA ScaledObjects
// to authenticate with Mimir for querying metrics.
func (c MonitoringConfig) IsKEDAAuthenticationEnabled(cluster *clusterv1.Cluster) bool {
	// Check global flag
	if !c.Enabled {
		return false
	}

	// If the cluster is being deleted, always return false
	deletionTimestamp := cluster.GetDeletionTimestamp()
	if deletionTimestamp != nil && !deletionTimestamp.IsZero() {
		return false
	}

	// Check cluster-specific label - must be explicitly set to "true" (defaults to false)
	labels := cluster.GetLabels()
	if labels == nil {
		return false // default to disabled when no labels
	}

	labelValue, ok := labels[KEDAAuthenticationLabel]
	if !ok {
		return false // default to disabled when label not set
	}

	// Only enabled if explicitly set to "true"
	return labelValue == "true"
}
