package config

import (
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

// isClusterFeatureEnabled is a utility function that checks if a feature is enabled for a specific cluster.
// A feature is enabled when all conditions are met:
//   - feature is enabled at the installation level (global flag)
//   - cluster is not being deleted
//   - cluster-specific feature label matches the expected value
//
// The defaultWhenMissing parameter controls the behavior when the label is missing:
//   - true: feature is enabled by default (opt-out model)
//   - false: feature is disabled by default (opt-in model)
func isClusterFeatureEnabled(globalEnabled bool, cluster *clusterv1.Cluster, labelKey string, defaultWhenMissing bool) bool {
	// Check global flag
	if !globalEnabled {
		return false
	}

	// If the cluster is being deleted, always return false
	deletionTimestamp := cluster.GetDeletionTimestamp()
	if deletionTimestamp != nil && !deletionTimestamp.IsZero() {
		return false
	}

	// Check cluster-specific label
	labels := cluster.GetLabels()
	if labels == nil {
		return defaultWhenMissing
	}

	labelValue, ok := labels[labelKey]
	if !ok {
		return defaultWhenMissing
	}

	// Check if label explicitly enables or disables the feature
	if defaultWhenMissing {
		// Opt-out model: disabled only if explicitly set to "false"
		return labelValue != "false"
	}
	// Opt-in model: enabled only if explicitly set to "true"
	return labelValue == "true"
}
