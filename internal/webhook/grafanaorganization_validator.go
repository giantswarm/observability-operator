/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webhook

import (
	"fmt"
	"slices"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
	observabilityv1alpha2 "github.com/giantswarm/observability-operator/api/v1alpha2"
)

// GrafanaOrganizationValidator provides shared validation logic for GrafanaOrganization resources
// across different API versions.
type GrafanaOrganizationValidator struct{}

// ValidateTenantIDs validates tenant IDs for v1alpha1 GrafanaOrganization.
// The CRD already validates: pattern (Alloy-compatible), minLength(1), maxLength(150), and minItems(1).
// This validator only adds: forbidden values and duplicates validation.
// See: https://grafana.com/docs/mimir/latest/configure/about-tenant-ids/
func (v *GrafanaOrganizationValidator) ValidateTenantIDs(tenantIDs []observabilityv1alpha1.TenantID) error {
	// List of forbidden tenant ID values that pass the CRD pattern but are not allowed by Mimir
	forbiddenValues := []string{"__mimir_cluster"}

	// Track seen tenant IDs to detect duplicates (CRD can't enforce uniqueness in arrays)
	seen := make(map[string]bool)

	for _, tenantID := range tenantIDs {
		tenantStr := string(tenantID)

		// Check for duplicates (CRD cannot enforce this)
		if seen[tenantStr] {
			return fmt.Errorf("duplicate tenant ID %q found", tenantStr)
		}
		seen[tenantStr] = true

		// Check forbidden values (CRD cannot enforce specific value exclusions)
		if slices.Contains(forbiddenValues, tenantStr) {
			return fmt.Errorf("tenant ID %q is not allowed. Forbidden values: %v", tenantStr, forbiddenValues)
		}
	}

	return nil
}

// ValidateTenantConfigs validates tenant configurations for v1alpha2 GrafanaOrganization.
// The CRD already validates: pattern (Alloy-compatible), minLength(1), maxLength(150), and minItems(1).
// This validator only adds: forbidden values and duplicates validation.
// See: https://grafana.com/docs/mimir/latest/configure/about-tenant-ids/
func (v *GrafanaOrganizationValidator) ValidateTenantConfigs(tenantConfigs []observabilityv1alpha2.TenantConfig) error {
	// List of forbidden tenant ID values that pass the CRD pattern but are not allowed by Mimir
	forbiddenValues := []string{"__mimir_cluster"}

	// Track seen tenant IDs to detect duplicates (CRD can't enforce uniqueness in arrays)
	seen := make(map[string]bool)

	for _, tenantConfig := range tenantConfigs {
		tenantStr := string(tenantConfig.Name)

		// Check for duplicates (CRD cannot enforce this)
		if seen[tenantStr] {
			return fmt.Errorf("duplicate tenant ID %q found", tenantStr)
		}
		seen[tenantStr] = true

		// Check forbidden values (CRD cannot enforce specific value exclusions)
		if slices.Contains(forbiddenValues, tenantStr) {
			return fmt.Errorf("tenant ID %q is not allowed. Forbidden values: %v", tenantStr, forbiddenValues)
		}

		// Validate tenant types if specified
		if err := v.validateTenantTypes(tenantConfig.Types); err != nil {
			return fmt.Errorf("invalid tenant types for tenant %q: %w", tenantStr, err)
		}
	}

	return nil
}

// validateTenantTypes validates that tenant types are valid and not empty
func (v *GrafanaOrganizationValidator) validateTenantTypes(types []observabilityv1alpha2.TenantType) error {
	if len(types) == 0 {
		// If no types specified, defaults to ["data"] as per CRD
		return nil
	}

	validTypes := []observabilityv1alpha2.TenantType{
		observabilityv1alpha2.TenantTypeData,
		observabilityv1alpha2.TenantTypeAlerting,
	}

	for _, tenantType := range types {
		if !slices.Contains(validTypes, tenantType) {
			return fmt.Errorf("invalid tenant type %q, valid types are: %v", tenantType, validTypes)
		}
	}

	return nil
}
