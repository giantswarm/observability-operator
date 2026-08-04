package logexport

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	"github.com/giantswarm/observability-operator/pkg/config"
)

const managementClusterName = "test-installation"

func TestGenerateAlloyLogExportConfigMapData(t *testing.T) {
	tests := []struct {
		name       string
		logExport  config.LogExportConfig
		goldenPath string
	}{
		{
			// The interim destination: an in-cluster S3-compatible store, which needs
			// an explicit endpoint and path-style addressing.
			name: "MinIO",
			logExport: config.LogExportConfig{
				Enabled:        true,
				Namespace:      "giantswarm",
				Endpoint:       "http://minio.alloy-logexporter.svc.cluster.local:9000",
				Bucket:         "audit-export",
				Region:         "us-east-1",
				Prefix:         "audit",
				ForcePathStyle: true,
			},
			goldenPath: filepath.Join("testdata", "logexport-config.MC.minio.yaml"),
		},
		{
			// A real bucket: no endpoint override, no path-style. Exercises that both
			// are omitted rather than emitted empty, which would be invalid config.
			name: "RealBucket",
			logExport: config.LogExportConfig{
				Enabled:   true,
				Namespace: "giantswarm",
				Bucket:    "customer-audit-archive",
				Region:    "eu-west-2",
				Prefix:    "giantswarm/audit",
			},
			goldenPath: filepath.Join("testdata", "logexport-config.MC.real-bucket.yaml"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Service{Config: config.Config{LogExport: tt.logExport}}

			data, err := s.GenerateAlloyLogExportConfigMapData()
			if err != nil {
				t.Fatalf("GenerateAlloyLogExportConfigMapData() error: %v", err)
			}

			result, ok := data["values"]
			if !ok {
				t.Fatalf("expected a %q key, got %v", "values", data)
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

			expected, err := os.ReadFile(tt.goldenPath)
			if err != nil {
				t.Fatalf("Failed to read golden file %s: %v", tt.goldenPath, err)
			}

			if diff := cmp.Diff(string(expected), result); diff != "" {
				t.Errorf("GenerateAlloyLogExportConfigMapData() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestIsLogExportEnabled covers the gating, which is the part most likely to cause
// harm if it is wrong: a false positive deploys an exporter that ships audit logs
// off-installation.
func TestIsLogExportEnabled(t *testing.T) {
	clusterConfig := config.ClusterConfig{Name: managementClusterName}

	cluster := func(name string, labels map[string]string) *clusterv1.Cluster {
		return &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "org-giantswarm", Labels: labels},
		}
	}

	deleting := func(name string) *clusterv1.Cluster {
		c := cluster(name, nil)
		now := metav1.Now()
		c.SetDeletionTimestamp(&now)
		c.SetFinalizers([]string{"test/finalizer"})
		return c
	}

	tests := []struct {
		name     string
		enabled  bool
		cluster  *clusterv1.Cluster
		expected bool
	}{
		{
			name:     "management cluster, flag on, no label",
			enabled:  true,
			cluster:  cluster(managementClusterName, nil),
			expected: true,
		},
		{
			name:     "management cluster, flag off",
			enabled:  false,
			cluster:  cluster(managementClusterName, nil),
			expected: false,
		},
		{
			// The exporter is installation-level: the Envoy mirror feeding it is
			// configured once at the gateway, so a workload cluster instance would
			// receive nothing.
			name:     "workload cluster is never enabled",
			enabled:  true,
			cluster:  cluster("some-workload-cluster", nil),
			expected: false,
		},
		{
			name:     "management cluster, label false",
			enabled:  true,
			cluster:  cluster(managementClusterName, map[string]string{config.LogExportLabel: "false"}),
			expected: false,
		},
		{
			name:     "management cluster, label true",
			enabled:  true,
			cluster:  cluster(managementClusterName, map[string]string{config.LogExportLabel: "true"}),
			expected: true,
		},
		{
			name:     "deleting cluster is never enabled",
			enabled:  true,
			cluster:  deleting(managementClusterName),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := config.LogExportConfig{Enabled: tt.enabled}
			if got := c.IsLogExportEnabled(tt.cluster, clusterConfig); got != tt.expected {
				t.Errorf("IsLogExportEnabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestGenerateAlloyLogExportSecretData asserts the refusal path: configuring the app
// without credentials would deploy something that silently drops everything.
func TestGenerateAlloyLogExportSecretData(t *testing.T) {
	s := &Service{}

	t.Setenv(envAccessKeyID, "")
	t.Setenv(envSecretAccessKey, "")
	if _, err := s.GenerateAlloyLogExportSecretData(); err == nil {
		t.Error("expected an error when credentials are missing, got nil")
	}

	t.Setenv(envAccessKeyID, "key")
	t.Setenv(envSecretAccessKey, "secret")
	got, err := s.GenerateAlloyLogExportSecretData()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{accessKeyIDKey: "key", secretAccessKeyKey: "secret"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("secret data mismatch (-want +got):\n%s", diff)
	}
}
