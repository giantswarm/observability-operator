package config

import (
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

// TODO rename to observability.giantswarm.io/tracing
const TracingLabel = "giantswarm.io/tracing"

// TracingConfig represents the configuration for tracing support in Grafana.
type TracingConfig struct {
	// Enabled controls tracing at the installation level
	Enabled bool
}

// Validate validates the tracing configuration
func (c TracingConfig) Validate() error {
	// Tracing config is always valid since it's just a boolean flag
	return nil
}

// IsTracingEnabled checks if tracing is enabled for a specific cluster.
// Tracing is enabled when all conditions are met:
//   - tracing is enabled at the installation level (global flag)
//   - cluster is not being deleted
//   - cluster-specific tracing label is set to true (or missing/invalid, defaulting to true)
func (c TracingConfig) IsTracingEnabled(cluster *clusterv1.Cluster) bool {
	return isClusterFeatureEnabled(c.Enabled, cluster, TracingLabel)
}
