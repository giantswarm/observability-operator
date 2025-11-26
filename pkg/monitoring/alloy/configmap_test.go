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
		shards                     int
	}{
		// Version 2.0.0+ tests (with extra query matchers, without scrape configs)
		{
			name: "OneShardTwoTenantsInWC_v200",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_multitenants.200.wc.river"),
			observabilityBundleVersion: semver.MustParse("2.0.0"),
			shards:                     1,
		},
		{
			name: "OneShardTwoTenantsInMC_v200",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_multitenants.200.mc.river"),
			observabilityBundleVersion: semver.MustParse("2.0.0"),
			shards:                     1,
		},
		{
			name: "OneShardSingleTenantInWC_v200",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_singletenant.200.wc.river"),
			observabilityBundleVersion: semver.MustParse("2.0.0"),
			shards:                     1,
		},
		{
			name: "OneShardSingleTenantInMC_v200",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_singletenant.200.mc.river"),
			observabilityBundleVersion: semver.MustParse("2.0.0"),
			shards:                     1,
		},
		{
			name: "OneShardDefaultTenantInWC_v200",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_defaulttenant.200.wc.river"),
			observabilityBundleVersion: semver.MustParse("2.0.0"),
			shards:                     1,
		},
		{
			name: "OneShardDefaultTenantInMC_v200",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_defaulttenant.200.mc.river"),
			observabilityBundleVersion: semver.MustParse("2.0.0"),
			shards:                     1,
		},

		// Version 2.0.0+ tests with 3 shards (with extra query matchers, without scrape configs)
		{
			name: "ThreeShardsTwoTenantsInWC_v200",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_threeshards_multitenants.200.wc.river"),
			observabilityBundleVersion: semver.MustParse("2.0.0"),
			shards:                     3,
		},
		{
			name: "ThreeShardsTwoTenantsInMC_v200",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_threeshards_multitenants.200.mc.river"),
			observabilityBundleVersion: semver.MustParse("2.0.0"),
			shards:                     3,
		},
		{
			name: "ThreeShardsSingleTenantInWC_v200",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_threeshards_singletenant.200.wc.river"),
			observabilityBundleVersion: semver.MustParse("2.0.0"),
			shards:                     3,
		},
		{
			name: "ThreeShardsSingleTenantInMC_v200",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_threeshards_singletenant.200.mc.river"),
			observabilityBundleVersion: semver.MustParse("2.0.0"),
			shards:                     3,
		},
		{
			name: "ThreeShardsDefaultTenantInWC_v200",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_threeshards_defaulttenant.200.wc.river"),
			observabilityBundleVersion: semver.MustParse("2.0.0"),
			shards:                     3,
		},
		{
			name: "ThreeShardsDefaultTenantInMC_v200",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_threeshards_defaulttenant.200.mc.river"),
			observabilityBundleVersion: semver.MustParse("2.0.0"),
			shards:                     3,
		},

		// Version 2.2.0+ tests (with extra query matchers and scrape configs)
		{
			name: "OneShardTwoTenantsInWC_v220",
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
			shards:                     1,
		},
		{
			name: "OneShardTwoTenantsInMC_v220",
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
			shards:                     1,
		},
		{
			name: "OneShardSingleTenantInWC_v220",
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
			shards:                     1,
		},
		{
			name: "OneShardSingleTenantInMC_v220",
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
			shards:                     1,
		},
		{
			name: "OneShardDefaultTenantInWC_v220",
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
			shards:                     1,
		},
		{
			name: "OneShardDefaultTenantInMC_v220",
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
			shards:                     1,
		},

		// Version 2.2.0+ tests with 3 shards (with extra query matchers and scrape configs)
		{
			name: "ThreeShardsTwoTenantsInWC_v220",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_threeshards_multitenants.220.wc.river"),
			observabilityBundleVersion: versionSupportingScrapeConfigs,
			shards:                     3,
		},
		{
			name: "ThreeShardsTwoTenantsInMC_v220",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_threeshards_multitenants.220.mc.river"),
			observabilityBundleVersion: versionSupportingScrapeConfigs,
			shards:                     3,
		},
		{
			name: "ThreeShardsSingleTenantInWC_v220",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_threeshards_singletenant.220.wc.river"),
			observabilityBundleVersion: versionSupportingScrapeConfigs,
			shards:                     3,
		},
		{
			name: "ThreeShardsSingleTenantInMC_v220",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_threeshards_singletenant.220.mc.river"),
			observabilityBundleVersion: versionSupportingScrapeConfigs,
			shards:                     3,
		},
		{
			name: "ThreeShardsDefaultTenantInWC_v220",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_threeshards_defaulttenant.220.wc.river"),
			observabilityBundleVersion: versionSupportingScrapeConfigs,
			shards:                     3,
		},
		{
			name: "ThreeShardsDefaultTenantInMC_v220",
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
			goldenPath:                 filepath.Join("testdata", "alloy_config_threeshards_defaulttenant.220.mc.river"),
			observabilityBundleVersion: versionSupportingScrapeConfigs,
			shards:                     3,
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

			got, err := service.generateAlloyConfig(ctx, tt.cluster, tt.tenants, tt.observabilityBundleVersion, tt.shards)
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
