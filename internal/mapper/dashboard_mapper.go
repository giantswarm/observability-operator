package mapper

import (
	"encoding/json"

	v1 "k8s.io/api/core/v1"

	"github.com/giantswarm/observability-operator/internal/labels"
	"github.com/giantswarm/observability-operator/pkg/domain/dashboard"
)

// DashboardMapper handles conversion from Kubernetes resources to domain objects
type DashboardMapper struct{}

// New creates a new mapper
func New() *DashboardMapper {
	return &DashboardMapper{}
}

// FromConfigMap converts a Kubernetes ConfigMap to domain Dashboard objects
func (m *DashboardMapper) FromConfigMap(cm *v1.ConfigMap) []*dashboard.Dashboard {
	org := m.extractOrganization(cm)
	folderPath := m.extractFolderPath(cm)

	var dashboards []*dashboard.Dashboard

	for _, dashboardString := range cm.Data {
		var content map[string]any
		if err := json.Unmarshal([]byte(dashboardString), &content); err != nil {
			// Create a dashboard with nil content for invalid JSON - let service layer handle validation
			dash := dashboard.New(org, folderPath, nil)
			dashboards = append(dashboards, dash)
			continue
		}

		dash := dashboard.New(org, folderPath, content)
		dashboards = append(dashboards, dash)
	}

	return dashboards
}

// extractOrganization returns the organization or empty string if not found.
func (m *DashboardMapper) extractOrganization(cm *v1.ConfigMap) string {
	annotations := cm.GetAnnotations()
	if annotations != nil && annotations[labels.GrafanaOrganizationKey] != "" {
		return annotations[labels.GrafanaOrganizationKey]
	}

	cmLabels := cm.GetLabels()
	if cmLabels != nil && cmLabels[labels.GrafanaOrganizationKey] != "" {
		return cmLabels[labels.GrafanaOrganizationKey]
	}

	return ""
}

// extractFolderPath returns the folder path or empty string (meaning "General" folder).
// Follows the same annotation-first, label-fallback pattern as extractOrganization.
func (m *DashboardMapper) extractFolderPath(cm *v1.ConfigMap) string {
	annotations := cm.GetAnnotations()
	if annotations != nil && annotations[labels.GrafanaFolderKey] != "" {
		return annotations[labels.GrafanaFolderKey]
	}

	cmLabels := cm.GetLabels()
	if cmLabels != nil && cmLabels[labels.GrafanaFolderKey] != "" {
		return cmLabels[labels.GrafanaFolderKey]
	}

	return ""
}
