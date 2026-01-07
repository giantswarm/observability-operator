package config

import (
	"fmt"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

// TODO rename to observability.giantswarm.io/logging
const LoggingLabel = "giantswarm.io/logging"

// LoggingConfig represents the configuration used by the logging package.
type LoggingConfig struct {
	// Enabled controls logging at the installation level
	Enabled bool

	// EnableAlloyEventsReconciliation controls events collection at the installation level
	// Disabled by default
	EnableAlloyEventsReconciliation bool

	// EnableNodeFiltering enables node filtering in Alloy logging configuration
	EnableNodeFiltering bool

	// EnableNetworkMonitoring enables network monitoring in Alloy logging configuration
	EnableNetworkMonitoring bool

	// DefaultNamespaces is the list of namespaces to collect logs from by default
	DefaultNamespaces []string

	// IncludeEventsNamespaces is the list of namespaces to collect events from
	// If empty, collect from all namespaces
	IncludeEventsNamespaces []string

	// ExcludeEventsNamespaces is the list of namespaces to exclude events from
	ExcludeEventsNamespaces []string
}

// Validate validates the logging configuration
func (l LoggingConfig) Validate() error {
	// Check for conflicting namespace configurations
	if len(l.IncludeEventsNamespaces) > 0 && len(l.ExcludeEventsNamespaces) > 0 {
		return fmt.Errorf("cannot specify both include and exclude events namespaces")
	}
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
