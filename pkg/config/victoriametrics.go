package config

import (
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

const VictoriaMetricsLabel = "giantswarm.io/victoriametrics"

// VictoriaMetricsConfig represents the configuration for Victoria Metrics support.
type VictoriaMetricsConfig struct {
	// Enabled controls Victoria Metrics at the installation level
	Enabled bool
}

// Validate validates the Victoria Metrics configuration
func (c VictoriaMetricsConfig) Validate() error {
	// Victoria Metrics config is always valid since it's just a boolean flag
	return nil
}

// IsVictoriaMetricsEnabled checks if Victoria Metrics is enabled for a specific cluster.
// Victoria Metrics is enabled when all conditions are met:
//   - Victoria Metrics is enabled at the installation level (global flag)
//   - cluster is not being deleted
//   - cluster-specific Victoria Metrics label is set to true (or missing/invalid, defaulting to true)
func (c VictoriaMetricsConfig) IsVictoriaMetricsEnabled(cluster *clusterv1.Cluster) bool {
	return isClusterFeatureEnabled(c.Enabled, cluster, VictoriaMetricsLabel)
}

