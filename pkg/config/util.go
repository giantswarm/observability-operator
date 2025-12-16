package config

import (
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

// isClusterFeatureEnabled is a utility function that checks if a feature is enabled for a specific cluster.
// A feature is enabled when all conditions are met:
//   - feature is enabled at the installation level (global flag)
//   - cluster is not being deleted
//   - cluster-specific feature label is set to true (or missing/invalid, defaulting to true)
func isClusterFeatureEnabled(globalEnabled bool, cluster *clusterv1.Cluster, labelKey string) bool {
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
		return true // default to enabled when no labels
	}

	labelValue, ok := labels[labelKey]
	if !ok {
		return true // default to enabled when label not set
	}

	// If label is set to "false", feature is disabled
	return labelValue != "false"
}
