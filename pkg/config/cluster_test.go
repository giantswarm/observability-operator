package config

import (
	"testing"

	"github.com/stretchr/testify/require"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func TestGetClusterProvider(t *testing.T) {
	tests := []struct {
		kind     string
		provider string
	}{
		{kind: AWSClusterKind, provider: "capa"},
		{kind: AWSManagedClusterKind, provider: "eks"},
		{kind: AzureClusterKind, provider: "capz"},
		{kind: AzureManagedClusterKind, provider: "aks"},
		{kind: AzureASOManagedClusterKind, provider: "aks"},
		{kind: VCDClusterKind, provider: "cloud-director"},
		{kind: VSphereClusterKind, provider: "vsphere"},
		{kind: GCPClusterKind, provider: "gcp"},
		{kind: GCPManagedClusterKind, provider: "gke"},
		{kind: ProxmoxClusterKind, provider: "proxmox"},
	}

	config := ClusterConfig{}
	for _, test := range tests {
		t.Run(test.kind, func(t *testing.T) {
			cluster := &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: test.kind,
					},
				},
			}
			provider, err := config.GetClusterProvider(cluster)
			require.NoError(t, err)
			require.Equal(t, test.provider, provider)
		})
	}

	t.Run("unknown kind", func(t *testing.T) {
		cluster := &clusterv1.Cluster{
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: clusterv1.ContractVersionedObjectReference{
					Kind: "DockerCluster",
				},
			},
		}
		_, err := config.GetClusterProvider(cluster)
		require.ErrorContains(t, err, "unknown cluster provider for DockerCluster")
	})
}
