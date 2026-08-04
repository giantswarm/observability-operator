package config

import (
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

const LogExportLabel = "observability.giantswarm.io/log-export"

// LogExportConfig represents the configuration used by the log export package.
//
// Log export tees audit and Teleport logs off the Loki write path into a
// customer-owned object store. Unlike the other collectors, the exporter is a
// single installation-level deployment on the management cluster -- the mirror
// that feeds it is configured once at the gateway, so a workload cluster
// instance would receive nothing.
type LogExportConfig struct {
	// Enabled controls log export at the installation level
	Enabled bool

	// Namespace is where the exporter's Helm values ConfigMap is written. It has
	// to match the namespace of the alloy-logexporter HelmRelease shipped by
	// management-cluster-bases, because a HelmRelease resolves valuesFrom
	// references in its own namespace only.
	Namespace string

	// Endpoint is the S3 endpoint to export to. Empty means the AWS default for
	// the configured region; set it to target an S3-compatible store.
	Endpoint string

	// Bucket is the destination bucket name.
	Bucket string

	// Region is the destination bucket's region.
	Region string

	// Prefix is the key prefix objects are written under.
	Prefix string

	// ForcePathStyle addresses the bucket as a path rather than a subdomain,
	// which S3-compatible stores generally require.
	ForcePathStyle bool
}

// IsLogExportEnabled checks whether log export should be configured for a cluster.
//
// It is enabled when all of these hold:
//   - log export is enabled at the installation level (global flag)
//   - the cluster is the management cluster -- the exporter is installation-level,
//     so a workload cluster never gets one
//   - the cluster is not being deleted
//   - the cluster-specific label is not set to false
//
// The installation flag is the switch, and it defaults to false, so nothing is
// exported by accident. The label only exists to turn export off again without
// changing the installation's configuration, which is why a missing label means
// enabled -- same as logging and monitoring.
func (c LogExportConfig) IsLogExportEnabled(cluster *clusterv1.Cluster, clusterConfig ClusterConfig) bool {
	if clusterConfig.IsWorkloadCluster(cluster) {
		return false
	}
	return isClusterFeatureEnabled(c.Enabled, cluster, LogExportLabel, true)
}
