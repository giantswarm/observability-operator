package config

import (
	"fmt"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

const (
	AWSClusterKind         = "AWSCluster"
	AWSClusterKindProvider = "capa"

	AWSManagedClusterKind         = "AWSManagedCluster"
	AWSManagedClusterKindProvider = "eks"

	AzureClusterKind         = "AzureCluster"
	AzureClusterKindProvider = "capz"

	AzureManagedClusterKind         = "AzureManagedCluster"
	AzureManagedClusterKindProvider = "aks"

	VCDClusterKind         = "VCDCluster"
	VCDClusterKindProvider = "cloud-director"

	VSphereClusterKind         = "VSphereCluster"
	VSphereClusterKindProvider = "vsphere"

	GCPClusterKind         = "GCPCluster"
	GCPClusterKindProvider = "gcp"

	GCPManagedClusterKind         = "GCPManagedCluster"
	GCPManagedClusterKindProvider = "gke"
)

// ClusterConfig represents the configuration for the management cluster.
type ClusterConfig struct {
	// BaseDomain is the base domain of the management cluster.
	BaseDomain string
	// Customer is the customer name of the management cluster.
	Customer string
	// InsecureCA is a flag to indicate if the management cluster has an insecure CA that should be trusted
	InsecureCA bool
	// Name is the name of the management cluster.
	Name string
	// Pipeline is the pipeline name of the management cluster.
	Pipeline string
	// Region is the region of the management cluster.
	Region string
}

// Validate validates the cluster configuration.
func (c ClusterConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("cluster name is required")
	}
	// Add other validation rules as needed
	return nil
}

// IsWorkloadCluster determines if the given cluster is a workload cluster (not the management cluster).
func (c ClusterConfig) IsWorkloadCluster(cluster *clusterv1.Cluster) bool {
	return cluster.Name != c.Name
}

// GetClusterType returns the type of the cluster (management_cluster or workload_cluster).
func (c ClusterConfig) GetClusterType(cluster *clusterv1.Cluster) string {
	if c.IsWorkloadCluster(cluster) {
		return "workload_cluster"
	}
	return "management_cluster"
}

// GetClusterProvider returns the provider for the given cluster.
func (c ClusterConfig) GetClusterProvider(cluster *clusterv1.Cluster) (string, error) {
	switch cluster.Spec.InfrastructureRef.Kind {
	case AWSClusterKind:
		return AWSClusterKindProvider, nil
	case AWSManagedClusterKind:
		return AWSManagedClusterKindProvider, nil
	case AzureClusterKind:
		return AzureClusterKindProvider, nil
	case AzureManagedClusterKind:
		return AzureManagedClusterKindProvider, nil
	case VCDClusterKind:
		return VCDClusterKindProvider, nil
	case VSphereClusterKind:
		return VSphereClusterKindProvider, nil
	case GCPClusterKind:
		return GCPClusterKindProvider, nil
	case GCPManagedClusterKind:
		return GCPManagedClusterKindProvider, nil
	}

	return "", fmt.Errorf("unknown cluster provider for %s", cluster.Spec.InfrastructureRef.Kind)
}
