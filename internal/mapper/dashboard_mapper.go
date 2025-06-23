package mapper

import (
	"encoding/json"

	v1 "k8s.io/api/core/v1"

	"github.com/giantswarm/observability-operator/pkg/domain/dashboard"
)

const grafanaOrganizationLabel = "observability.giantswarm.io/organization"

// DashboardMapper handles conversion from Kubernetes resources to domain objects
type DashboardMapper struct{}

// New creates a new mapper
func New() *DashboardMapper {
	return &DashboardMapper{}
}

// FromConfigMap converts a Kubernetes ConfigMap to domain Dashboard objects
func (m *DashboardMapper) FromConfigMap(cm *v1.ConfigMap) []*dashboard.Dashboard {
	org := m.extractOrganization(cm)

	var dashboards []*dashboard.Dashboard

	for _, dashboardString := range cm.Data {
		var content map[string]any
		if err := json.Unmarshal([]byte(dashboardString), &content); err != nil {
			// Create a dashboard with nil content for invalid JSON - let service layer handle validation
			dash := dashboard.New("", org, nil)
			dashboards = append(dashboards, dash)
			continue
		}

		uid := m.extractUID(content)
		dash := dashboard.New(uid, org, content)
		dashboards = append(dashboards, dash)
	}

	return dashboards
}

// extractOrganization - returns organization or empty string if not found
func (m *DashboardMapper) extractOrganization(cm *v1.ConfigMap) string {
	// Try to look for an annotation first
	annotations := cm.GetAnnotations()
	if annotations != nil && annotations[grafanaOrganizationLabel] != "" {
		return annotations[grafanaOrganizationLabel]
	}

	// Then look for a label
	labels := cm.GetLabels()
	if labels != nil && labels[grafanaOrganizationLabel] != "" {
		return labels[grafanaOrganizationLabel]
	}

	return ""
}

func (m *DashboardMapper) extractUID(content map[string]any) string {
	uid, ok := content["uid"].(string)
	if !ok {
		return ""
	}
	return uid
}
