package events

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/giantswarm/observability-operator/pkg/common/organization/mocks"
	"github.com/giantswarm/observability-operator/pkg/common/tenancy"
	"github.com/giantswarm/observability-operator/pkg/config"
)

var (
	managementClusterName = "test-installation"
)

func TestGenerateAlloyEventsConfig(t *testing.T) {
	tests := []struct {
		name              string
		cluster           *clusterv1.Cluster
		tenants           []string
		goldenPath        string
		loggingEnabled    bool
		tracingEnabled    bool
		monitoringEnabled bool
		includeNamespaces []string
		excludeNamespaces []string
	}{
		{
			name: "ManagementCluster_LokiEvents",
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
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			tenants:        []string{"giantswarm"},
			goldenPath:     filepath.Join("testdata", "events-logger-config.alloy.MC.loki-events.yaml"),
			loggingEnabled: true,
			tracingEnabled: false,
		},
		{
			name: "WorkloadCluster_LokiEvents",
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
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			tenants:        []string{"giantswarm"},
			goldenPath:     filepath.Join("testdata", "events-logger-config.alloy.WC.loki-events.yaml"),
			loggingEnabled: true,
			tracingEnabled: false,
		},
		{
			name: "WorkloadCluster_LokiEventsIncludeNamespaces",
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
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			tenants:           []string{"giantswarm"},
			goldenPath:        filepath.Join("testdata", "events-logger-config.alloy.WC.loki-events-include-namespaces.yaml"),
			loggingEnabled:    true,
			tracingEnabled:    false,
			includeNamespaces: []string{"namespace1", "namespace2"},
		},
		{
			name: "WorkloadCluster_LokiEventsExcludeNamespaces",
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
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			tenants:           []string{"giantswarm"},
			goldenPath:        filepath.Join("testdata", "events-logger-config.alloy.WC.loki-events-exclude-namespaces.yaml"),
			loggingEnabled:    true,
			tracingEnabled:    false,
			excludeNamespaces: []string{"namespace1", "namespace2"},
		},
		{
			name: "ManagementCluster_OTLPTraces",
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
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			tenants:        []string{"giantswarm"},
			goldenPath:     filepath.Join("testdata", "events-logger-config.alloy.MC.tracing-enabled.yaml"),
			loggingEnabled: false,
			tracingEnabled: true,
		},
		{
			name: "WorkloadCluster_OTLPTraces",
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
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			tenants:        []string{"giantswarm"},
			goldenPath:     filepath.Join("testdata", "events-logger-config.alloy.WC.tracing-enabled.yaml"),
			loggingEnabled: false,
			tracingEnabled: true,
		},
		{
			name: "ManagementCluster_NoneEnabled",
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
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			tenants: []string{"giantswarm"},
			// goldenPath omitted - this should return an error
			loggingEnabled: false,
			tracingEnabled: false,
		},
		{
			name: "WorkloadCluster_NoneEnabled",
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
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			tenants: []string{"giantswarm"},
			// goldenPath omitted - this should return an error
			loggingEnabled: false,
			tracingEnabled: false,
		},
		{
			name: "ManagementCluster_OTLPMetrics",
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
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			tenants:           []string{"giantswarm"},
			goldenPath:        filepath.Join("testdata", "events-logger-config.alloy.MC.monitoring-enabled.yaml"),
			loggingEnabled:    false,
			tracingEnabled:    false,
			monitoringEnabled: true,
		},
		{
			name: "WorkloadCluster_OTLPMetrics",
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
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			tenants:           []string{"giantswarm"},
			goldenPath:        filepath.Join("testdata", "events-logger-config.alloy.WC.monitoring-enabled.yaml"),
			loggingEnabled:    false,
			tracingEnabled:    false,
			monitoringEnabled: true,
		},
		{
			name: "ManagementCluster_AllSignals",
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
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			tenants:           []string{"giantswarm"},
			goldenPath:        filepath.Join("testdata", "events-logger-config.alloy.MC.all-signals.yaml"),
			loggingEnabled:    true,
			tracingEnabled:    true,
			monitoringEnabled: true,
		},
		{
			name: "WorkloadCluster_AllSignals",
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
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						Kind: "AWSCluster",
					},
				},
			},
			tenants:           []string{"giantswarm"},
			goldenPath:        filepath.Join("testdata", "events-logger-config.alloy.WC.all-signals.yaml"),
			loggingEnabled:    true,
			tracingEnabled:    true,
			monitoringEnabled: true,
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
					LokiMaxBackoffPeriod:    "10m",
					LokiRemoteTimeout:       "60s",
				},
				OTLP: config.OTLPConfig{
					BatchSendBatchSize: 1024,
					BatchMaxSize:       1024,
					BatchTimeout:       "500ms",
				},
				DefaultTenant: "giantswarm",
			}

			mockOrgRepo := mocks.NewMockOrganizationRepository("test-organization")
			service := &Service{
				Config:                 cfg,
				OrganizationRepository: mockOrgRepo,
				TenantRepository:       tenancy.NewTenantRepository(fakeClient),
			}

			// Generate Alloy events config using the actual service method
			resultMap, err := service.GenerateAlloyEventsConfigMapData(
				ctx,
				tt.cluster,
				tt.loggingEnabled,
				tt.tracingEnabled,
				tt.monitoringEnabled,
			)

			// Check if this is a "neither enabled" test case (no golden path)
			if tt.goldenPath == "" {
				// Should return an error when no feature is enabled
				if err == nil {
					t.Errorf("GenerateAlloyEventsConfigMapData() expected error when neither logging nor tracing nor monitoring enabled, got nil")
				}
				return
			}

			// For valid test cases, no error should occur
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
