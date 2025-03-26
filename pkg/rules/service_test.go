package rules

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	commonmonitoring "github.com/giantswarm/observability-operator/pkg/common/monitoring"
)

func TestGenerateAlloyConfig(t *testing.T) {
	tests := []struct {
		name       string
		tenants    []string
		goldenPath string
	}{
		{
			name:       "TwoTenants",
			tenants:    []string{"tenant1", "tenant2"},
			goldenPath: filepath.Join("testdata", "alloy_config_multitenants.river"),
		},
		{
			name:       "SingleTenant",
			tenants:    []string{"tenant1"},
			goldenPath: filepath.Join("testdata", "alloy_config_singletenant.river"),
		},
		{
			name:       "DefaultTenantRendersLegacyConfig",
			tenants:    []string{commonmonitoring.DefaultWriteTenant},
			goldenPath: filepath.Join("testdata", "alloy_config_defaulttenant.river"),
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			// Create a dummy Service with minimal dependencies.
			service := &Service{}

			got, err := service.generateAlloyConfig(ctx, tt.tenants)
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
