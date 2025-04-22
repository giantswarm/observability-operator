package alloy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blang/semver/v4"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/giantswarm/observability-operator/pkg/common"
	commonmonitoring "github.com/giantswarm/observability-operator/pkg/common/monitoring"
	"github.com/giantswarm/observability-operator/pkg/monitoring"
)

var managementClusterName = "dummy-cluster"

// dummyOrgRepo implements a minimal OrganizationRepository.
type dummyOrgRepo struct{}

func (d *dummyOrgRepo) Read(ctx context.Context, cluster *clusterv1.Cluster) (string, error) {
	return "dummy-org", nil
}

func TestGenerateAlloyConfig(t *testing.T) {
	tests := []struct {
		name                       string
		cluster                    *clusterv1.Cluster
		tenants                    []string
		goldenPath                 string
		observabilityBundleVersion semver.Version
	}{
		{
			name: "TwoTenantsInWC",
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
			tenants:                    []string{"tenant1", "tenant2"},
			goldenPath:                 filepath.Join("testdata", "alloy_config_multitenants.wc.river"),
			observabilityBundleVersion: versionSupportingExtraQueryMatchers,
		},
		{
			name: "TwoTenantsInMC",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managementClusterName,
					Namespace: "default",
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			tenants:                    []string{"tenant1", "tenant2"},
			goldenPath:                 filepath.Join("testdata", "alloy_config_multitenants.mc.river"),
			observabilityBundleVersion: versionSupportingExtraQueryMatchers,
		},
		{
			name: "SingleTenantInWC",
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
			tenants:                    []string{"tenant1"},
			goldenPath:                 filepath.Join("testdata", "alloy_config_singletenant.wc.river"),
			observabilityBundleVersion: versionSupportingExtraQueryMatchers,
		},
		{
			name: "SingleTenantInMC",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managementClusterName,
					Namespace: "default",
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "AzureCluster",
					},
				},
			},
			tenants:                    []string{"tenant1"},
			goldenPath:                 filepath.Join("testdata", "alloy_config_singletenant.mc.river"),
			observabilityBundleVersion: versionSupportingExtraQueryMatchers,
		},
		{
			name: "DefaultTenantRendersLegacyConfigInWC",
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
			tenants:                    []string{commonmonitoring.DefaultWriteTenant},
			goldenPath:                 filepath.Join("testdata", "alloy_config_defaulttenant.wc.river"),
			observabilityBundleVersion: versionSupportingExtraQueryMatchers,
		},
		{
			name: "DefaultTenantRendersLegacyConfigInMC",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managementClusterName,
					Namespace: "default",
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "AzureCluster",
					},
				},
			},
			tenants:                    []string{commonmonitoring.DefaultWriteTenant},
			goldenPath:                 filepath.Join("testdata", "alloy_config_defaulttenant.mc.river"),
			observabilityBundleVersion: versionSupportingExtraQueryMatchers,
		},

		// Test case for the old bundle version to make sure we do not render extra query matchers in versions < 1.9.0
		{
			name: "TwoTenantsWithOldBundleVersionInMC",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managementClusterName,
					Namespace: "default",
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			tenants:                    []string{"tenant1", "tenant2"},
			goldenPath:                 filepath.Join("testdata", "alloy_config_multitenants.170.mc.river"),
			observabilityBundleVersion: semver.MustParse("1.7.0"),
		},
		{
			name: "TwoTenantsWithNewBundleVersionInMC",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managementClusterName,
					Namespace: "default",
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			tenants:                    []string{"tenant1", "tenant2"},
			goldenPath:                 filepath.Join("testdata", "alloy_config_multitenants.190.mc.river"),
			observabilityBundleVersion: versionSupportingExtraQueryMatchers,
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

			got, err := service.generateAlloyConfig(ctx, tt.cluster, tt.tenants, tt.observabilityBundleVersion)
			if err != nil {
				t.Fatalf("generateAlloyConfig failed: %v", err)
			}

			if os.Getenv("UPDATE_GOLDEN_FILES") == "true" {
				t.Logf("Environment variable UPDATE_GOLDEN_FILES=true detected, updating golden files")
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
