package config

import (
	"strconv"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

const TracingLabel = "giantswarm.io/tracing"

// TracingConfig represents the configuration for tracing support in Grafana.
type TracingConfig struct {
	Enabled bool
}

// IsTracingEnabled should be enabled when all conditions are met:
//   - global tracing flag is enabled
//   - tracing label is not set or is set to true on the cluster object
func (c TracingConfig) IsTracingEnabled(cluster *clusterv1.Cluster) bool {
	if !c.Enabled {
		return false
	}

	// Check if label is set on the cluster object
	labels := cluster.GetLabels()
	tracingLabelValue, ok := labels[TracingLabel]
	if !ok {
		// If it's not set, tracing is enabled by default
		return true
	}

	tracingEnabled, err := strconv.ParseBool(tracingLabelValue)
	if err != nil {
		return true
	}
	return tracingEnabled
}

// Validate validates the tracing configuration
func (c TracingConfig) Validate() error {
	// Tracing config is always valid since it's just a boolean flag
	return nil
}
