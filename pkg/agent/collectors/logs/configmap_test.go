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
		loggingEnabled             bool
		nodeFilteringEnabled       bool
		networkMonitoringEnabled   bool
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
			goldenPath:                 filepath.Join("testdata", "logging-config.alloy.170.MC.basic.yaml"),
			observabilityBundleVersion: semver.MustParse("1.7.0"),
			loggingEnabled:             true,
			nodeFilteringEnabled:       false,
			networkMonitoringEnabled:   false,
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
			goldenPath:                 filepath.Join("testdata", "logging-config.alloy.170.WC.basic.yaml"),
			observabilityBundleVersion: semver.MustParse("1.7.0"),
			loggingEnabled:             true,
			nodeFilteringEnabled:       false,
			networkMonitoringEnabled:   false,
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
			goldenPath:                 filepath.Join("testdata", "logging-config.alloy.170.WC.default-namespaces-nil.yaml"),
			observabilityBundleVersion: semver.MustParse("1.7.0"),
			loggingEnabled:             true,
			nodeFilteringEnabled:       false,
			networkMonitoringEnabled:   false,
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
			goldenPath:                 filepath.Join("testdata", "logging-config.alloy.170.WC.default-namespaces-empty.yaml"),
			observabilityBundleVersion: semver.MustParse("1.7.0"),
			loggingEnabled:             true,
			nodeFilteringEnabled:       false,
			networkMonitoringEnabled:   false,
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
			goldenPath:                 filepath.Join("testdata", "logging-config.alloy.170.WC.custom-tenants.yaml"),
			observabilityBundleVersion: semver.MustParse("1.7.0"),
			loggingEnabled:             true,
			nodeFilteringEnabled:       false,
			networkMonitoringEnabled:   false,
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
			goldenPath:                 filepath.Join("testdata", "logging-config.alloy.170.MC.node-filtering.yaml"),
			observabilityBundleVersion: semver.MustParse("1.7.0"),
			loggingEnabled:             true,
			nodeFilteringEnabled:       true,
			networkMonitoringEnabled:   false,
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
			goldenPath:                 filepath.Join("testdata", "logging-config.alloy.170.WC.node-filtering.yaml"),
			observabilityBundleVersion: semver.MustParse("1.7.0"),
			loggingEnabled:             true,
			nodeFilteringEnabled:       true,
			networkMonitoringEnabled:   false,
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
			goldenPath:                 filepath.Join("testdata", "logging-config.alloy.240.WC.node-filtering.yaml"),
			observabilityBundleVersion: semver.MustParse("2.4.0"),
			loggingEnabled:             true,
			nodeFilteringEnabled:       true,
			networkMonitoringEnabled:   false,
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
			goldenPath:                 filepath.Join("testdata", "logging-config.alloy.230.MC.network-monitoring.yaml"),
			observabilityBundleVersion: semver.MustParse("2.3.0"),
			loggingEnabled:             true,
			nodeFilteringEnabled:       false,
			networkMonitoringEnabled:   true,
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
			goldenPath:                 filepath.Join("testdata", "logging-config.alloy.230.WC.network-monitoring.yaml"),
			observabilityBundleVersion: semver.MustParse("2.3.0"),
			loggingEnabled:             true,
			nodeFilteringEnabled:       false,
			networkMonitoringEnabled:   true,
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
			goldenPath:                 filepath.Join("testdata", "logging-config.alloy.230.WC.network-monitoring-node-filtering.yaml"),
			observabilityBundleVersion: semver.MustParse("2.3.0"),
			loggingEnabled:             true,
			nodeFilteringEnabled:       true,
			networkMonitoringEnabled:   true,
		},
		{
			name: "ManagementCluster_NetworkMonitoringOnly",
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
			goldenPath:                 filepath.Join("testdata", "logging-config.alloy.230.MC.network-monitoring-only.yaml"),
			observabilityBundleVersion: semver.MustParse("2.3.0"),
			loggingEnabled:             false,
			nodeFilteringEnabled:       false,
			networkMonitoringEnabled:   true,
		},
		{
			name: "WorkloadCluster_NetworkMonitoringOnly",
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
			goldenPath:                 filepath.Join("testdata", "logging-config.alloy.230.WC.network-monitoring-only.yaml"),
			observabilityBundleVersion: semver.MustParse("2.3.0"),
			loggingEnabled:             false,
			nodeFilteringEnabled:       false,
			networkMonitoringEnabled:   true,
		},
		{
			name: "WorkloadCluster_NeitherEnabled",
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
			tenants:           []string{"giantswarm"},
			defaultNamespaces: []string{"test-selector"},
			// goldenPath omitted - this should return an error
			observabilityBundleVersion: semver.MustParse("2.3.0"),
			loggingEnabled:             false,
			nodeFilteringEnabled:       false,
			networkMonitoringEnabled:   false,
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
					Enabled:             tt.loggingEnabled,
					DefaultNamespaces:   tt.defaultNamespaces,
					EnableNodeFiltering: tt.nodeFilteringEnabled,
				},
				Monitoring: config.MonitoringConfig{
					NetworkEnabled: tt.networkMonitoringEnabled,
				},
			}

			mockOrgRepo := mocks.NewMockOrganizationRepository("test-organization")
			service := &Service{
				Config:                 cfg,
				OrganizationRepository: mockOrgRepo,
				TenantRepository:       tenancy.NewTenantRepository(fakeClient),
			}

			// Generate Alloy logs config using the actual service method
			resultMap, err := service.GenerateAlloyLogsConfigMapData(
				ctx,
				tt.cluster,
				tt.observabilityBundleVersion,
				tt.loggingEnabled,
				tt.networkMonitoringEnabled,
			)

			// Check if this is a "neither enabled" test case (no golden path)
			if tt.goldenPath == "" {
				// Should return an error when neither feature is enabled
				if err == nil {
					t.Errorf("GenerateAlloyLogsConfigMapData() expected error when neither logging nor network monitoring enabled, got nil")
				}
				return
			}

			// For valid test cases, no error should occur
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
