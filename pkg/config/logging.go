package config

import (
	"strconv"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

const LoggingLabel = "giantswarm.io/logging"

// LoggingConfig represents the configuration used by the logging package.
type LoggingConfig struct {
	Enabled bool
}

// IsLoggingEnabled should be enabled when all conditions are met:
//   - global logging flag is enabled
//   - logging label is not set or is set to true on the cluster object
func (c LoggingConfig) IsLoggingEnabled(cluster *clusterv1.Cluster) bool {
	if !c.Enabled {
		return false
	}

	// Check if label is set on the cluster object
	labels := cluster.GetLabels()
	loggingLabelValue, ok := labels[LoggingLabel]
	if !ok {
		// If it's not set, logging is enabled by default
		return true
	}

	loggingEnabled, err := strconv.ParseBool(loggingLabelValue)
	if err != nil {
		return true
	}
	return loggingEnabled
}

// Validate validates the logging configuration
func (c LoggingConfig) Validate() error {
	// Logging config is always valid since it's just a boolean flag
	return nil
}
