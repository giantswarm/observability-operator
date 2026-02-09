package metrics

import (
	"strings"
	"testing"

	"github.com/giantswarm/observability-operator/pkg/agent/common"
)

func TestGenerateKEDAExtraObjects(t *testing.T) {
	secretData := map[string]string{
		common.MimirRemoteWriteAPIUsernameKey: "test-cluster",
		common.MimirRemoteWriteAPIPasswordKey: "test-password",
	}

	tests := []struct {
		name          string
		kedaNamespace string
		wantCTA       bool
		wantSecret    bool
	}{
		{
			name:          "with default namespace creates ClusterTriggerAuthentication and Secret",
			kedaNamespace: "keda",
			wantCTA:       true,
			wantSecret:    true,
		},
		{
			name:          "with custom namespace creates ClusterTriggerAuthentication and Secret",
			kedaNamespace: "keda-system",
			wantCTA:       true,
			wantSecret:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := generateKEDAExtraObjects(tt.kedaNamespace, secretData)
			if err != nil {
				t.Fatalf("generateKEDAExtraObjects() error = %v", err)
			}

			if tt.wantCTA {
				if !strings.Contains(result, "kind: ClusterTriggerAuthentication") {
					t.Error("expected ClusterTriggerAuthentication in output")
				}
				if !strings.Contains(result, "name: giantswarm-mimir-auth") {
					t.Error("expected giantswarm-mimir-auth name in output")
				}
				if !strings.Contains(result, "name: alloy-metrics") {
					t.Error("expected alloy-metrics secret reference in output")
				}
			}

			if tt.wantSecret {
				if !strings.Contains(result, "kind: Secret") {
					t.Error("expected Secret in output")
				}
				if !strings.Contains(result, "namespace: "+tt.kedaNamespace) {
					t.Errorf("expected namespace %s in output", tt.kedaNamespace)
				}
				if !strings.Contains(result, "test-cluster") {
					t.Error("expected username value in output")
				}
				if !strings.Contains(result, "test-password") {
					t.Error("expected password value in output")
				}
			} else {
				if strings.Contains(result, "kind: Secret") {
					t.Error("unexpected Secret in output")
				}
			}
		})
	}
}
