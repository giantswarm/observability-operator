package config

import (
	"fmt"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

const LoggingLabel = "observability.giantswarm.io/logging"
const LegacyLoggingLabel = "giantswarm.io/logging"

// LoggingConfig represents the configuration used by the logging package.
type LoggingConfig struct {
	// Enabled controls logging at the installation level
	Enabled bool

	// OTLPEnabled controls whether OTLP log ingestion is enabled via the events collector
	OTLPEnabled bool

	// EnableNodeFiltering enables node filtering in Alloy logging configuration
	EnableNodeFiltering bool

	// DefaultNamespaces is the list of namespaces to collect logs from by default
	DefaultNamespaces []string

	// IncludeEventsNamespaces is the list of namespaces to collect events from
	// If empty, collect from all namespaces
	IncludeEventsNamespaces []string

	// ExcludeEventsNamespaces is the list of namespaces to exclude events from
	ExcludeEventsNamespaces []string

	// RulerURL is the URL to the Loki ruler API used to clean up rules on cluster deletion.
	// Leave empty to disable Loki ruler cleanup.
	RulerURL string
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
	return isClusterFeatureEnabled(l.Enabled, cluster, LoggingLabel, LegacyLoggingLabel, true)
}
