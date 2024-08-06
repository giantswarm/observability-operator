package common

import (
	"errors"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
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

	MonitoringAgentPrometheus = "prometheus-agent"
	MonitoringAgentAlloy      = "alloy"

	MonitoringPrometheusAgentAppName = "prometheusAgent"
	MonitoringAlloyAppName           = "alloyMetrics"
)

type ManagementCluster struct {
	// BaseDomain is the base domain of the management cluster.
	BaseDomain string
	// Customer is the customer name of the management cluster.
	Customer string
	// InsecureCA is a flag to indicate if the management cluster has an insecure CA that should be truster
	InsecureCA bool
	// Name is the name of the management cluster.
	Name string
	// Pipeline is the pipeline name of the management cluster.
	Pipeline string
	// Region is the region of the management cluster.
	Region string
}

func GetClusterType(cluster *clusterv1.Cluster, mc ManagementCluster) string {
	if cluster.Name == mc.Name {
		return "management_cluster"
	}
	return "workload_cluster"
}

func GetClusterProvider(cluster *clusterv1.Cluster) (string, error) {
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

	return "", errors.New("unknown cluster provider")
}
