package tenancy

import (
	"context"
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/observability-operator/api/v1alpha1"
)

const (
	TenantSelectorLabel = "observability.giantswarm.io/tenant"
)

func ListTenants(ctx context.Context, client client.Client) ([]string, error) {
	tenants := make([]string, 0)
	var grafanaOrganizations v1alpha1.GrafanaOrganizationList

	err := client.List(ctx, &grafanaOrganizations)
	if err != nil {
		return nil, err
	}

	for _, organization := range grafanaOrganizations.Items {
		if !organization.DeletionTimestamp.IsZero() {
			continue
		}

		for _, tenant := range organization.Spec.Tenants {
			if !slices.Contains(tenants, string(tenant)) {
				tenants = append(tenants, string(tenant))
			}
		}
	}

	return tenants, nil
}
