package events

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

func TestGenerateAlloyEventsConfig(t *testing.T) {
	tests := []struct {
		name                       string
		cluster                    *clusterv1.Cluster
		tenants                    []string
		goldenPath                 string
		observabilityBundleVersion semver.Version
		tracingEnabled             bool
		includeNamespaces          []string
		excludeNamespaces          []string
	}{
		{
			name: "ManagementCluster_NoTracing",
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
			goldenPath:                 filepath.Join("testdata", "events-logger-config.alloy.MC.yaml"),
			observabilityBundleVersion: semver.MustParse("1.7.0"),
			tracingEnabled:             false,
		},
		{
			name: "WorkloadCluster_NoTracing",
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
			goldenPath:                 filepath.Join("testdata", "events-logger-config.alloy.WC.yaml"),
			observabilityBundleVersion: semver.MustParse("1.7.0"),
			tracingEnabled:             false,
		},
		{
			name: "WorkloadCluster_IncludeNamespaces",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "include-namespaces",
					Namespace: "default",
					Labels: map[string]string{
						"giantswarm.io/cluster":     "include-namespaces",
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
			goldenPath:                 filepath.Join("testdata", "events-logger-config.alloy.WC.include-namespaces.yaml"),
			observabilityBundleVersion: semver.MustParse("1.7.0"),
			tracingEnabled:             false,
			includeNamespaces:          []string{"namespace1", "namespace2"},
		},
		{
			name: "WorkloadCluster_ExcludeNamespaces",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "exclude-namespaces",
					Namespace: "default",
					Labels: map[string]string{
						"giantswarm.io/cluster":     "exclude-namespaces",
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
			goldenPath:                 filepath.Join("testdata", "events-logger-config.alloy.WC.exclude-namespaces.yaml"),
			observabilityBundleVersion: semver.MustParse("1.7.0"),
			tracingEnabled:             false,
			excludeNamespaces:          []string{"namespace1", "namespace2"},
		},
		{
			name: "ManagementCluster_TracingEnabled",
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
			goldenPath:                 filepath.Join("testdata", "events-logger-config.alloy.MC.tracing-enabled.yaml"),
			observabilityBundleVersion: semver.MustParse("1.11.0"),
			tracingEnabled:             true,
		},
		{
			name: "WorkloadCluster_TracingEnabled",
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
			goldenPath:                 filepath.Join("testdata", "events-logger-config.alloy.WC.tracing-enabled.yaml"),
			observabilityBundleVersion: semver.MustParse("1.11.0"),
			tracingEnabled:             true,
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

			grafanaOrg := &v1alpha1.GrafanaOrganization{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-org",
					Namespace: "default",
				},
				Spec: v1alpha1.GrafanaOrganizationSpec{
					Tenants: []v1alpha1.TenantID{"giantswarm"},
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
					IncludeEventsNamespaces: tt.includeNamespaces,
					ExcludeEventsNamespaces: tt.excludeNamespaces,
				},
			}

			mockOrgRepo := mocks.NewMockOrganizationRepository("test-organization")
			service := &Service{
				ConfigurationRepository: agent.NewConfigurationRepository(fakeClient),
				Config:                  cfg,
				OrganizationRepository:  mockOrgRepo,
				TenantRepository:        tenancy.NewTenantRepository(fakeClient),
			}

			// Generate Alloy events config using the actual service method
			resultMap, err := service.GenerateAlloyEventsConfigMapData(
				ctx,
				tt.cluster,
				tt.tracingEnabled,
				tt.observabilityBundleVersion,
			)
			if err != nil {
				t.Fatalf("GenerateAlloyEventsConfigMapData() failed: %v", err)
			}

			// Extract the values from the map
			result, ok := resultMap["values"]
			if !ok {
				t.Fatalf("GenerateAlloyEventsConfigMapData() did not return 'values' key")
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
				t.Errorf("GenerateAlloyEventsConfigMapData() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
