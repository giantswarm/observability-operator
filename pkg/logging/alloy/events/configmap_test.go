package events

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blang/semver/v4"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

	"github.com/giantswarm/observability-operator/pkg/common/organization/mocks"
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

			// Create service with test configuration
			cfg := config.Config{
				Cluster: config.ClusterConfig{
					BaseDomain: "test.gigantic.io",
				},
				Logging: config.LoggingConfig{
					IncludeEventsNamespaces: tt.includeNamespaces,
					ExcludeEventsNamespaces: tt.excludeNamespaces,
				},
			}

			mockOrgRepo := mocks.NewMockOrganizationRepository("test-organization")
			service := &Service{
				Config:                 cfg,
				OrganizationRepository: mockOrgRepo,
			}

			// Get cluster metadata
			clusterID := tt.cluster.Name
			installation := managementClusterName
			if _, ok := tt.cluster.Labels["giantswarm.io/cluster"]; ok {
				installation = managementClusterName
			}
			organization := "test-organization"
			provider := "test-provider"
			clusterType := "workload_cluster"
			if installation == clusterID {
				clusterType = "management_cluster"
			}
			isWorkloadCluster := clusterType == "workload_cluster"

			tempoURL := ""
			if tt.tracingEnabled && isWorkloadCluster {
				tempoURL = "tempo-gateway.test.gigantic.io"
			}

			// Generate Alloy events config
			result, err := service.generateAlloyEventsConfig(
				clusterID,
				clusterType,
				organization,
				provider,
				tempoURL,
				tt.tenants,
				tt.tracingEnabled,
				isWorkloadCluster,
			)
			if err != nil {
				t.Fatalf("generateAlloyEventsConfig() failed: %v", err)
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
				t.Errorf("generateAlloyEventsConfig() mismatch (-want +got):\n%s", diff)
			}

			_ = ctx
		})
	}
}
