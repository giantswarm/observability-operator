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

	commonmonitoring "github.com/giantswarm/observability-operator/pkg/common/monitoring"
	"github.com/giantswarm/observability-operator/pkg/config"
)

var managementClusterName = "dummy-cluster"

// MockOrganizationRepository implements a minimal OrganizationRepository with call tracking.
type MockOrganizationRepository struct {
	CallCount   int
	LastCluster *clusterv1.Cluster
}

func (m *MockOrganizationRepository) Read(ctx context.Context, cluster *clusterv1.Cluster) (string, error) {
	m.CallCount++
	m.LastCluster = cluster
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
		// Version 2.0.0+ tests (with extra query matchers, without scrape configs)
		{
			name: "TwoTenantsInWC_v200",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_multitenants.190.wc.river"),
			observabilityBundleVersion: semver.MustParse("2.0.0"),
		},
		{
			name: "TwoTenantsInMC_v200",
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
			observabilityBundleVersion: semver.MustParse("2.0.0"),
		},
		{
			name: "SingleTenantInWC_v190",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_singletenant.190.wc.river"),
			observabilityBundleVersion: semver.MustParse("2.0.0"),
		},
		{
			name: "SingleTenantInMC_v190",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_singletenant.190.mc.river"),
			observabilityBundleVersion: semver.MustParse("2.0.0"),
		},
		{
			name: "DefaultTenantInWC_v190",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_defaulttenant.190.wc.river"),
			observabilityBundleVersion: semver.MustParse("2.0.0"),
		},
		{
			name: "DefaultTenantInMC_v190",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_defaulttenant.190.mc.river"),
			observabilityBundleVersion: semver.MustParse("2.0.0"),
		},

		// Version 2.2.0+ tests (with extra query matchers and scrape configs)
		{
			name: "TwoTenantsInWC_v220",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_multitenants.220.wc.river"),
			observabilityBundleVersion: versionSupportingScrapeConfigs,
		},
		{
			name: "TwoTenantsInMC_v220",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_multitenants.220.mc.river"),
			observabilityBundleVersion: versionSupportingScrapeConfigs,
		},
		{
			name: "SingleTenantInWC_v220",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_singletenant.220.wc.river"),
			observabilityBundleVersion: versionSupportingScrapeConfigs,
		},
		{
			name: "SingleTenantInMC_v220",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_singletenant.220.mc.river"),
			observabilityBundleVersion: versionSupportingScrapeConfigs,
		},
		{
			name: "DefaultTenantInWC_v220",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_defaulttenant.220.wc.river"),
			observabilityBundleVersion: versionSupportingScrapeConfigs,
		},
		{
			name: "DefaultTenantInMC_v220",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_defaulttenant.220.mc.river"),
			observabilityBundleVersion: versionSupportingScrapeConfigs,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			// Create a dummy Service with minimal dependencies.
			service := &Service{
				OrganizationRepository: &MockOrganizationRepository{},
				Config: config.Config{
					Cluster: config.ClusterConfig{
						InsecureCA: false,
						Customer:   "dummy-customer",
						Name:       "dummy-cluster",
						Pipeline:   "dummy-pipeline",
						Region:     "dummy-region",
					},
					Monitoring: config.MonitoringConfig{
						WALTruncateFrequency: time.Minute,
						QueueConfig: config.QueueConfig{
							Capacity:          &[]int{30000}[0],
							MaxShards:         &[]int{10}[0],
							MaxSamplesPerSend: &[]int{150000}[0],
							SampleAgeLimit:    &[]string{"30m"}[0],
						},
					},
				},
			}

			got, err := service.generateAlloyConfig(ctx, tt.cluster, tt.tenants, tt.observabilityBundleVersion)
			if err != nil {
				t.Fatalf("generateAlloyConfig failed: %v", err)
			}

			if os.Getenv("UPDATE_GOLDEN_FILES") == "true" {
				t.Logf("Environment variable UPDATE_GOLDEN_FILES=true detected, updating golden files")
				if err := os.MkdirAll(filepath.Dir(tt.goldenPath), 0750); err != nil {
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
