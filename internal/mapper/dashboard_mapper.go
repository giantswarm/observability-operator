package mapper

import (
	"encoding/json"
	"fmt"

	"github.com/giantswarm/observability-operator/pkg/domain/dashboard"
	v1 "k8s.io/api/core/v1"
)

const grafanaOrganizationLabel = "observability.giantswarm.io/organization"

// DashboardMapper handles conversion from Kubernetes resources to domain objects
type DashboardMapper struct{}

// New creates a new mapper
func New() *DashboardMapper {
	return &DashboardMapper{}
}

// FromConfigMap converts a Kubernetes ConfigMap to domain Dashboard objects
func (m *DashboardMapper) FromConfigMap(cm *v1.ConfigMap) ([]*dashboard.Dashboard, error) {
	org, err := m.extractOrganization(cm)
	if err != nil {
		return nil, err
	}

	var dashboards []*dashboard.Dashboard

	for _, dashboardString := range cm.Data {
		var content map[string]any
		if err := json.Unmarshal([]byte(dashboardString), &content); err != nil {
			return nil, fmt.Errorf("%w: %v", dashboard.ErrInvalidJSON, err)
		}

		uid, err := m.extractUID(content)
		if err != nil {
			return nil, err
		}

		dash := dashboard.New(uid, org, content)
		dashboards = append(dashboards, dash)
	}

	return dashboards, nil
}

// extractOrganization - exact copy of existing getOrgFromDashboardConfigmap
func (m *DashboardMapper) extractOrganization(cm *v1.ConfigMap) (string, error) {
	// Try to look for an annotation first
	annotations := cm.GetAnnotations()
	if annotations != nil && annotations[grafanaOrganizationLabel] != "" {
		return annotations[grafanaOrganizationLabel], nil
	}

	// Then look for a label
	labels := cm.GetLabels()
	if labels != nil && labels[grafanaOrganizationLabel] != "" {
		return labels[grafanaOrganizationLabel], nil
	}

	return "", dashboard.ErrMissingOrganization
}

func (m *DashboardMapper) extractUID(content map[string]any) (string, error) {
	uid, ok := content["uid"].(string)
	if !ok {
		return "", dashboard.ErrMissingUID
	}
	return uid, nil
}
