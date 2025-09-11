package metrics

import (
	"context"
	"fmt"
	"strconv"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/observability-operator/api/v1alpha1"
)

const (
	ActiveStatus  = "active"
	PendingStatus = "pending"
	ErrorStatus   = "error"
)

// GrafanaOrganizationCollector collects metrics for GrafanaOrganization resources
type GrafanaOrganizationCollector struct {
	client client.Client
}

// NewGrafanaOrganizationCollector creates a new GrafanaOrganizationCollector
func NewGrafanaOrganizationCollector(client client.Client) *GrafanaOrganizationCollector {
	return &GrafanaOrganizationCollector{
		client: client,
	}
}

// CollectMetrics collects and updates all GrafanaOrganization metrics
func (c *GrafanaOrganizationCollector) CollectMetrics(ctx context.Context) error {
	var organizations v1alpha1.GrafanaOrganizationList
	if err := c.client.List(ctx, &organizations); err != nil {
		return fmt.Errorf("failed to list GrafanaOrganization resources: %w", err)
	}

	// Reset all gauge metrics
	GrafanaOrganizationTotal.Reset()
	GrafanaOrganizationTenants.Reset()
	GrafanaOrganizationInfo.Reset()

	statusCounts := map[string]int{
		ActiveStatus:  0,
		PendingStatus: 0,
		ErrorStatus:   0,
	}

	for _, org := range organizations.Items {
		orgName := org.Name
		orgIDStr := strconv.FormatInt(org.Status.OrgID, 10)
		displayName := org.Spec.DisplayName

		// Determine status
		status := PendingStatus
		if org.Status.OrgID > 0 {
			status = ActiveStatus
		}

		statusCounts[status]++

		// Update gauge metrics
		GrafanaOrganizationTenants.WithLabelValues(orgName, orgIDStr).Set(float64(len(org.Spec.Tenants)))

		// Set info metrics
		GrafanaOrganizationInfo.WithLabelValues(orgName, displayName, orgIDStr).Set(1)
	}

	// Update total counts by status
	for status, count := range statusCounts {
		GrafanaOrganizationTotal.WithLabelValues(status).Set(float64(count))
	}

	return nil
}
