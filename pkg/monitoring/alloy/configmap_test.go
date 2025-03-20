package alloy

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/giantswarm/observability-operator/pkg/common"
	commonmonitoring "github.com/giantswarm/observability-operator/pkg/common/monitoring"
	"github.com/giantswarm/observability-operator/pkg/monitoring"
)

var (
	update = flag.Bool("update", false, "update .golden files")
)

// dummyOrgRepo implements a minimal OrganizationRepository.
type dummyOrgRepo struct{}

func (d *dummyOrgRepo) Read(ctx context.Context, cluster *clusterv1.Cluster) (string, error) {
	return "dummy-org", nil
}

func TestGenerateAlloyConfig(t *testing.T) {
	tests := []struct {
		name       string
		cluster    *clusterv1.Cluster
		tenants    []string
		goldenPath string
	}{
		{
			name: "TwoTenants",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			tenants:    []string{"tenant1", "tenant2"},
			goldenPath: filepath.Join("testdata", "alloy_config_multitenants.river"),
		},
		{
			name: "SingleTenant",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "single-tenant-cluster",
					Namespace: "default",
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "AzureCluster",
					},
				},
			},
			tenants:    []string{"tenant1"},
			goldenPath: filepath.Join("testdata", "alloy_config_singletenant.river"),
		},
		{
			name: "DefaultTenantRendersLegacyConfig",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-tenant-cluster",
					Namespace: "default",
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "AzureCluster",
					},
				},
			},
			tenants:    []string{commonmonitoring.DefaultWriteTenant},
			goldenPath: filepath.Join("testdata", "alloy_config_defaulttenant.river"),
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			// Create a dummy Service with minimal dependencies.
			service := &Service{
				OrganizationRepository: &dummyOrgRepo{},
				ManagementCluster: common.ManagementCluster{
					InsecureCA: false,
					Customer:   "dummy-customer",
					Name:       "dummy-cluster",
					Pipeline:   "dummy-pipeline",
					Region:     "dummy-region",
				},
				MonitoringConfig: monitoring.Config{
					WALTruncateFrequency: time.Minute,
				},
			}

			got, err := service.generateAlloyConfig(ctx, tt.cluster, tt.tenants)
			if err != nil {
				t.Fatalf("generateAlloyConfig failed: %v", err)
			}

			if *update {
				if err := os.MkdirAll(filepath.Dir(tt.goldenPath), 0755); err != nil {
					t.Fatalf("failed to create golden directory: %v", err)
				}
				//nolint:gosec
				if err := os.WriteFile(tt.goldenPath, []byte(got), 0644); err != nil {
					t.Fatalf("failed to update golden file: %v", err)
				}
			}

			wantBytes, err := os.ReadFile(tt.goldenPath)
			if err != nil {
				t.Fatalf("failed to read golden file: %v", err)
			}
			want := string(wantBytes)
			if got != want {
				t.Errorf("generated config does not match golden file for %s.\nGot:\n%s\n\nWant:\n%s", tt.name, got, want)
			}
		})
	}
}
