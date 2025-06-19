package mapper

import (
	"testing"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
  
	"github.com/giantswarm/observability-operator/pkg/domain/dashboard"
)

func TestNew(t *testing.T) {
	dashboardMapper := New()
	if dashboardMapper == nil {
		t.Error("Expected New to return a non-nil mapper")
	}
}

func TestFromConfigMap(t *testing.T) {
	tests := []struct {
		name              string
		configMap         *v1.ConfigMap
		expectedCount     int
		expectError       bool
		expectedErrorType error
	}{
		{
			name: "valid configmap with single dashboard",
			configMap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dashboard",
					Namespace: "default",
					Labels: map[string]string{
						"observability.giantswarm.io/organization": "test-org",
					},
				},
				Data: map[string]string{
					"dashboard.json": `{"uid": "test-uid", "title": "Test Dashboard"}`,
				},
			},
			expectedCount: 1,
			expectError:   false,
		},
		{
			name: "valid configmap with annotation organization",
			configMap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dashboard",
					Namespace: "default",
					Annotations: map[string]string{
						"observability.giantswarm.io/organization": "test-org",
					},
				},
				Data: map[string]string{
					"dashboard.json": `{"uid": "test-uid", "title": "Test Dashboard"}`,
				},
			},
			expectedCount: 1,
			expectError:   false,
		},
		{
			name: "multiple dashboards in configmap",
			configMap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dashboard",
					Namespace: "default",
					Labels: map[string]string{
						"observability.giantswarm.io/organization": "test-org",
					},
				},
				Data: map[string]string{
					"dashboard1.json": `{"uid": "test-uid-1", "title": "Test Dashboard 1"}`,
					"dashboard2.json": `{"uid": "test-uid-2", "title": "Test Dashboard 2"}`,
				},
			},
			expectedCount: 2,
			expectError:   false,
		},
		{
			name: "missing organization",
			configMap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dashboard",
					Namespace: "default",
				},
				Data: map[string]string{
					"dashboard.json": `{"uid": "test-uid", "title": "Test Dashboard"}`,
				},
			},
			expectedCount:     0,
			expectError:       true,
			expectedErrorType: dashboard.ErrMissingOrganization,
		},
		{
			name: "invalid JSON",
			configMap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dashboard",
					Namespace: "default",
					Labels: map[string]string{
						"observability.giantswarm.io/organization": "test-org",
					},
				},
				Data: map[string]string{
					"dashboard.json": `invalid json`,
				},
			},
			expectedCount:     0,
			expectError:       true,
			expectedErrorType: dashboard.ErrInvalidJSON,
		},
		{
			name: "missing UID in dashboard",
			configMap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dashboard",
					Namespace: "default",
					Labels: map[string]string{
						"observability.giantswarm.io/organization": "test-org",
					},
				},
				Data: map[string]string{
					"dashboard.json": `{"title": "Test Dashboard"}`,
				},
			},
			expectedCount:     0,
			expectError:       true,
			expectedErrorType: dashboard.ErrMissingUID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dashboardMapper := New()
			dashboards, err := dashboardMapper.FromConfigMap(tt.configMap)

			if tt.expectError {
				if err == nil {
					t.Error("Expected an error but got none")
				}
				if tt.expectedErrorType != nil && !IsError(err, tt.expectedErrorType) {
					t.Errorf("Expected error type %v, got %v", tt.expectedErrorType, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if len(dashboards) != tt.expectedCount {
					t.Errorf("Expected %d dashboards, got %d", tt.expectedCount, len(dashboards))
				}

				// Verify dashboard properties for successful cases
				for i, dash := range dashboards {
					if dash.Organization() != "test-org" {
						t.Errorf("Dashboard %d: expected organization 'test-org', got '%s'", i, dash.Organization())
					}
					if dash.UID() == "" {
						t.Errorf("Dashboard %d: expected non-empty UID", i)
					}
				}
			}
		})
	}
}

func TestFromConfigMapEdgeCases(t *testing.T) {
	dashboardMapper := New()

	t.Run("empty configmap data", func(t *testing.T) {
		cm := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"observability.giantswarm.io/organization": "test-org",
				},
			},
			Data: map[string]string{},
		}

		dashboards, err := dashboardMapper.FromConfigMap(cm)
		if err != nil {
			t.Errorf("Expected no error for empty data, got: %v", err)
		}
		if len(dashboards) != 0 {
			t.Errorf("Expected 0 dashboards for empty data, got %d", len(dashboards))
		}
	})

	t.Run("both label and annotation present - annotation takes precedence", func(t *testing.T) {
		cm := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"observability.giantswarm.io/organization": "label-org",
				},
				Annotations: map[string]string{
					"observability.giantswarm.io/organization": "annotation-org",
				},
			},
			Data: map[string]string{
				"dashboard.json": `{"uid": "test-uid", "title": "Test Dashboard"}`,
			},
		}

		dashboards, err := dashboardMapper.FromConfigMap(cm)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if len(dashboards) != 1 {
			t.Errorf("Expected 1 dashboard, got %d", len(dashboards))
		}
		if dashboards[0].Organization() != "annotation-org" {
			t.Errorf("Expected organization 'annotation-org', got '%s'", dashboards[0].Organization())
		}
	})
}

// Helper function to check if an error matches a specific type
func IsError(err, target error) bool {
	if err == nil || target == nil {
		return err == target
	}
	return err.Error() == target.Error() ||
		(len(err.Error()) >= len(target.Error()) &&
			err.Error()[:len(target.Error())] == target.Error())
}
