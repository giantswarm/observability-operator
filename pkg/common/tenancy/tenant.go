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

// ListTenants retrieves a unique, sorted list of tenant IDs from all active GrafanaOrganization resources.
func ListTenants(ctx context.Context, k8sClient client.Client) ([]string, error) {
	var grafanaOrganizations v1alpha1.GrafanaOrganizationList
	if err := k8sClient.List(ctx, &grafanaOrganizations); err != nil {
		return nil, err
	}

	// Use a map to store unique tenants for efficient lookup.
	// The map value (struct{}{}) is an empty struct, which uses minimal memory.
	uniqueTenants := make(map[string]struct{})

	for _, organization := range grafanaOrganizations.Items {
		// Skip organizations marked for deletion.
		if !organization.DeletionTimestamp.IsZero() {
			continue
		}

		for _, tenant := range organization.Spec.Tenants {
			uniqueTenants[string(tenant)] = struct{}{}
		}
	}

	// Convert map keys to a slice.
	tenants := make([]string, 0, len(uniqueTenants))
	for tenant := range uniqueTenants {
		tenants = append(tenants, tenant)
	}

	// Sort the tenants for deterministic output.
	slices.Sort(tenants)

	return tenants, nil
}
