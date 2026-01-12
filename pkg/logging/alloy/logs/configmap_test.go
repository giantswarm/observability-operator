package logs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blang/semver/v4"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/giantswarm/observability-operator/pkg/agent"
	"github.com/giantswarm/observability-operator/pkg/common/organization/mocks"
	"github.com/giantswarm/observability-operator/pkg/common/tenancy"
	"github.com/giantswarm/observability-operator/pkg/config"
)

var (
	managementClusterName = "test-installation"
)

func TestGenerateAlloyLogsConfig(t *testing.T) {
	tests := []struct {
		name                       string
		cluster                    *clusterv1.Cluster
		tenants                    []string
		defaultNamespaces          []string
		goldenPath                 string
		observabilityBundleVersion semver.Version
		enableNodeFiltering        bool
		enableNetworkMonitoring    bool
	}{
		{
			name: "ManagementCluster_Basic",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managementClusterName,
					Namespace: "default",
					Labels: map[string]string{
						"giantswarm.io/cluster":     managementClusterName,
						"cluster.x-k8s.io/provider": "aws",
					},
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			tenants:                    []string{"giantswarm"},
			defaultNamespaces:          []string{"test-selector"},
			goldenPath:                 filepath.Join("testdata", "logging-config.alloy.170_MC.yaml"),
			observabilityBundleVersion: semver.MustParse("1.7.0"),
			enableNodeFiltering:        false,
			enableNetworkMonitoring:    false,
		},
		{
			name: "WorkloadCluster_Basic",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
					Labels: map[string]string{
						"giantswarm.io/cluster":     "test-cluster",
						"cluster.x-k8s.io/provider": "aws",
					},
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			tenants:                    []string{"giantswarm"},
			defaultNamespaces:          []string{"test-selector"},
			goldenPath:                 filepath.Join("testdata", "logging-config.alloy.170_WC.yaml"),
			observabilityBundleVersion: semver.MustParse("1.7.0"),
			enableNodeFiltering:        false,
			enableNetworkMonitoring:    false,
		},
		{
			name: "WorkloadCluster_DefaultNamespacesNil",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
					Labels: map[string]string{
						"giantswarm.io/cluster":     "test-cluster",
						"cluster.x-k8s.io/provider": "aws",
					},
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			tenants:                    []string{"giantswarm"},
			defaultNamespaces:          nil,
			goldenPath:                 filepath.Join("testdata", "logging-config.alloy.170_WC_default_namespaces_nil.yaml"),
			observabilityBundleVersion: semver.MustParse("1.7.0"),
			enableNodeFiltering:        false,
			enableNetworkMonitoring:    false,
		},
		{
			name: "WorkloadCluster_DefaultNamespacesEmpty",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
					Labels: map[string]string{
						"giantswarm.io/cluster":     "test-cluster",
						"cluster.x-k8s.io/provider": "aws",
					},
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			tenants:                    []string{"giantswarm"},
			defaultNamespaces:          []string{""},
			goldenPath:                 filepath.Join("testdata", "logging-config.alloy.170_WC_default_namespaces_empty.yaml"),
			observabilityBundleVersion: semver.MustParse("1.7.0"),
			enableNodeFiltering:        false,
			enableNetworkMonitoring:    false,
		},
		{
			name: "WorkloadCluster_CustomTenants",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
					Labels: map[string]string{
						"giantswarm.io/cluster":     "test-cluster",
						"cluster.x-k8s.io/provider": "aws",
					},
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			tenants:                    []string{"test-tenant-a", "test-tenant-b"},
			defaultNamespaces:          []string{""},
			goldenPath:                 filepath.Join("testdata", "logging-config.alloy.170_WC_custom_tenants.yaml"),
			observabilityBundleVersion: semver.MustParse("1.7.0"),
			enableNodeFiltering:        false,
			enableNetworkMonitoring:    false,
		},
		{
			name: "ManagementCluster_NodeFiltering",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managementClusterName,
					Namespace: "default",
					Labels: map[string]string{
						"giantswarm.io/cluster":     managementClusterName,
						"cluster.x-k8s.io/provider": "aws",
					},
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			tenants:                    []string{"giantswarm"},
			defaultNamespaces:          []string{"test-selector"},
			goldenPath:                 filepath.Join("testdata", "logging-config.alloy.170_MC_node_filtering.yaml"),
			observabilityBundleVersion: semver.MustParse("1.7.0"),
			enableNodeFiltering:        true,
			enableNetworkMonitoring:    false,
		},
		{
			name: "WorkloadCluster_NodeFiltering",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
					Labels: map[string]string{
						"giantswarm.io/cluster":     "test-cluster",
						"cluster.x-k8s.io/provider": "aws",
					},
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			tenants:                    []string{"giantswarm"},
			defaultNamespaces:          []string{"test-selector"},
			goldenPath:                 filepath.Join("testdata", "logging-config.alloy.170_WC_node_filtering.yaml"),
			observabilityBundleVersion: semver.MustParse("1.7.0"),
			enableNodeFiltering:        true,
			enableNetworkMonitoring:    false,
		},
		{
			name: "WorkloadCluster_NodeFiltering_v240",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
					Labels: map[string]string{
						"giantswarm.io/cluster":     "test-cluster",
						"cluster.x-k8s.io/provider": "aws",
					},
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			tenants:                    []string{"giantswarm"},
			defaultNamespaces:          []string{"test-selector"},
			goldenPath:                 filepath.Join("testdata", "logging-config.alloy.240_WC_node_filtering.yaml"),
			observabilityBundleVersion: semver.MustParse("2.4.0"),
			enableNodeFiltering:        true,
			enableNetworkMonitoring:    false,
		},
		{
			name: "ManagementCluster_NetworkMonitoring",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managementClusterName,
					Namespace: "default",
					Labels: map[string]string{
						"giantswarm.io/cluster":     managementClusterName,
						"cluster.x-k8s.io/provider": "aws",
					},
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			tenants:                    []string{"giantswarm"},
			defaultNamespaces:          []string{"test-selector"},
			goldenPath:                 filepath.Join("testdata", "logging-config.alloy.230_MC_network_monitoring.yaml"),
			observabilityBundleVersion: semver.MustParse("2.3.0"),
			enableNodeFiltering:        false,
			enableNetworkMonitoring:    true,
		},
		{
			name: "WorkloadCluster_NetworkMonitoring",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
					Labels: map[string]string{
						"giantswarm.io/cluster":     "test-cluster",
						"cluster.x-k8s.io/provider": "aws",
					},
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			tenants:                    []string{"giantswarm"},
			defaultNamespaces:          []string{"test-selector"},
			goldenPath:                 filepath.Join("testdata", "logging-config.alloy.230_WC_network_monitoring.yaml"),
			observabilityBundleVersion: semver.MustParse("2.3.0"),
			enableNodeFiltering:        false,
			enableNetworkMonitoring:    true,
		},
		{
			name: "WorkloadCluster_NetworkMonitoring_NodeFiltering",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
					Labels: map[string]string{
						"giantswarm.io/cluster":     "test-cluster",
						"cluster.x-k8s.io/provider": "aws",
					},
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			tenants:                    []string{"giantswarm"},
			defaultNamespaces:          []string{"test-selector"},
			goldenPath:                 filepath.Join("testdata", "logging-config.alloy.230_WC_network_monitoring_node_filtering.yaml"),
			observabilityBundleVersion: semver.MustParse("2.3.0"),
			enableNodeFiltering:        true,
			enableNetworkMonitoring:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			// Create a fake client with a GrafanaOrganization to provide tenants
			scheme := runtime.NewScheme()
			_ = v1alpha1.AddToScheme(scheme)
			_ = clusterv1.AddToScheme(scheme)

			// Convert tenant strings to TenantID type
			tenantIDs := make([]v1alpha1.TenantID, len(tt.tenants))
			for i, t := range tt.tenants {
				tenantIDs[i] = v1alpha1.TenantID(t)
			}

			grafanaOrg := &v1alpha1.GrafanaOrganization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-org",
					Namespace: "default",
				},
				Spec: v1alpha1.GrafanaOrganizationSpec{
					Tenants: tenantIDs,
				},
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(grafanaOrg).
				Build()

			// Create service with test configuration
			cfg := config.Config{
				Cluster: config.ClusterConfig{
					BaseDomain: "test.gigantic.io",
					Name:       managementClusterName,
				},
				Logging: config.LoggingConfig{
					DefaultNamespaces:   tt.defaultNamespaces,
					EnableNodeFiltering: tt.enableNodeFiltering,
				},
				Monitoring: config.MonitoringConfig{
					NetworkEnabled: tt.enableNetworkMonitoring,
				},
			}

			mockOrgRepo := mocks.NewMockOrganizationRepository("test-organization")
			service := &Service{
				ConfigurationRepository: agent.NewConfigurationRepository(fakeClient),
				Config:                  cfg,
				OrganizationRepository:  mockOrgRepo,
				TenantRepository:        tenancy.NewTenantRepository(fakeClient),
			}

			// Generate Alloy logs config using the actual service method
			resultMap, err := service.GenerateAlloyLogsConfigMapData(
				ctx,
				tt.cluster,
				tt.observabilityBundleVersion,
				tt.enableNetworkMonitoring,
			)
			if err != nil {
				t.Fatalf("GenerateAlloyLogsConfigMapData() failed: %v", err)
			}

			// Extract the values from the map
			result, ok := resultMap["values"]
			if !ok {
				t.Fatalf("GenerateAlloyLogsConfigMapData() did not return 'values' key")
			}

			if os.Getenv("UPDATE_GOLDEN_FILES") == "true" {
				t.Logf("Environment variable UPDATE_GOLDEN_FILES=true detected, updating golden files")
				if err := os.MkdirAll(filepath.Dir(tt.goldenPath), 0750); err != nil {
					t.Fatalf("failed to create golden directory: %v", err)
				}
				//nolint:gosec
				if err := os.WriteFile(tt.goldenPath, []byte(result), 0644); err != nil {
					t.Fatalf("failed to update golden file: %v", err)
				}
			}

			// Read expected output from golden file
			expected, err := os.ReadFile(tt.goldenPath)
			if err != nil {
				t.Fatalf("Failed to read golden file %s: %v", tt.goldenPath, err)
			}

			// Compare
			if diff := cmp.Diff(string(expected), result); diff != "" {
				t.Errorf("GenerateAlloyLogsConfigMapData() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
