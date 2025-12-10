package config

import (
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

// TODO rename to observability.giantswarm.io/logging
const LoggingLabel = "giantswarm.io/logging"

// LoggingConfig represents the configuration used by the logging package.
type LoggingConfig struct {
	// Enabled controls logging at the installation level
	Enabled bool
}

// Validate validates the logging configuration
func (l LoggingConfig) Validate() error {
	// Logging config is always valid since it's just a boolean flag
	return nil
}

// IsLoggingEnabled checks if logging is enabled for a specific cluster.
// Logging is enabled when all conditions are met:
//   - logging is enabled at the installation level (global flag)
//   - cluster is not being deleted
//   - cluster-specific logging label is set to true (or missing/invalid, defaulting to true)
func (l LoggingConfig) IsLoggingEnabled(cluster *clusterv1.Cluster) bool {
	return isClusterFeatureEnabled(l.Enabled, cluster, LoggingLabel)
}
