package mapper

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/giantswarm/observability-operator/internal/labels"
)

func TestNew(t *testing.T) {
	dashboardMapper := New()
	if dashboardMapper == nil {
		t.Error("Expected New to return a non-nil mapper")
	}
}

func TestFromConfigMap(t *testing.T) {
	tests := []struct {
		name          string
		configMap     *v1.ConfigMap
		expectedCount int
	}{
		{
			name: "valid configmap with single dashboard",
			configMap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dashboard",
					Namespace: "default",
					Labels: map[string]string{
						labels.GrafanaOrganizationAnnotation: "test-org",
					},
				},
				Data: map[string]string{
					"dashboard.json": `{"uid": "test-uid", "title": "Test Dashboard"}`,
				},
			},
			expectedCount: 1,
		},
		{
			name: "valid configmap with annotation organization",
			configMap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dashboard",
					Namespace: "default",
					Annotations: map[string]string{
						labels.GrafanaOrganizationAnnotation: "test-org",
					},
				},
				Data: map[string]string{
					"dashboard.json": `{"uid": "test-uid", "title": "Test Dashboard"}`,
				},
			},
			expectedCount: 1,
		},
		{
			name: "multiple dashboards in configmap",
			configMap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dashboard",
					Namespace: "default",
					Labels: map[string]string{
						labels.GrafanaOrganizationAnnotation: "test-org",
					},
				},
				Data: map[string]string{
					"dashboard1.json": `{"uid": "test-uid-1", "title": "Test Dashboard 1"}`,
					"dashboard2.json": `{"uid": "test-uid-2", "title": "Test Dashboard 2"}`,
				},
			},
			expectedCount: 2,
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
			expectedCount: 1,
		},
		{
			name: "invalid JSON",
			configMap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dashboard",
					Namespace: "default",
					Labels: map[string]string{
						labels.GrafanaOrganizationAnnotation: "test-org",
					},
				},
				Data: map[string]string{
					"dashboard.json": `invalid json`,
				},
			},
			expectedCount: 1,
		},
		{
			name: "missing UID in dashboard",
			configMap: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dashboard",
					Namespace: "default",
					Labels: map[string]string{
						labels.GrafanaOrganizationAnnotation: "test-org",
					},
				},
				Data: map[string]string{
					"dashboard.json": `{"title": "Test Dashboard"}`,
				},
			},
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dashboardMapper := New()
			dashboards := dashboardMapper.FromConfigMap(tt.configMap)

			if len(dashboards) != tt.expectedCount {
				t.Errorf("Expected %d dashboards, got %d", tt.expectedCount, len(dashboards))
			}

			// Verify dashboard properties for successful cases with valid data
			for i, dash := range dashboards {
				// Only check organization for cases where we expect it to be set
				if tt.configMap.Annotations != nil && tt.configMap.Annotations[labels.GrafanaOrganizationAnnotation] != "" {
					if dash.Organization() != tt.configMap.Annotations[labels.GrafanaOrganizationAnnotation] {
						t.Errorf("Dashboard %d: expected organization from annotation, got '%s'", i, dash.Organization())
					}
				} else if tt.configMap.Labels != nil && tt.configMap.Labels[labels.GrafanaOrganizationAnnotation] != "" {
					if dash.Organization() != tt.configMap.Labels[labels.GrafanaOrganizationAnnotation] {
						t.Errorf("Dashboard %d: expected organization from label, got '%s'", i, dash.Organization())
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
					labels.GrafanaOrganizationAnnotation: "test-org",
				},
			},
			Data: map[string]string{},
		}

		dashboards := dashboardMapper.FromConfigMap(cm)
		if len(dashboards) != 0 {
			t.Errorf("Expected 0 dashboards for empty data, got %d", len(dashboards))
		}
	})

	t.Run("both label and annotation present - annotation takes precedence", func(t *testing.T) {
		cm := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					labels.GrafanaOrganizationAnnotation: "label-org",
				},
				Annotations: map[string]string{
					labels.GrafanaOrganizationAnnotation: "annotation-org",
				},
			},
			Data: map[string]string{
				"dashboard.json": `{"uid": "test-uid", "title": "Test Dashboard"}`,
			},
		}

		dashboards := dashboardMapper.FromConfigMap(cm)
		if len(dashboards) != 1 {
			t.Errorf("Expected 1 dashboard, got %d", len(dashboards))
		}
		if dashboards[0].Organization() != "annotation-org" {
			t.Errorf("Expected organization 'annotation-org', got '%s'", dashboards[0].Organization())
		}
	})
}
