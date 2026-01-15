// Package tenancy provides functionality for managing tenant information across the observability platform.
//
// The package defines a TenantRepository interface for retrieving tenant IDs and provides
// a Kubernetes-based implementation that extracts tenants from GrafanaOrganization resources.
//
// Tenants represent isolated units within the multi-tenant observability platform and are used
// for filtering and organizing observability data (metrics, logs, traces) across different
// organizational boundaries.
//
// Example usage:
//
//	tenantRepo := tenancy.NewKubernetesRepository(k8sClient)
//	tenants, err := tenantRepo.List(ctx)
//	if err != nil {
//	    return fmt.Errorf("failed to list tenants: %w", err)
//	}
//	// tenants contains a sorted, deduplicated list of tenant IDs
package tenancy
