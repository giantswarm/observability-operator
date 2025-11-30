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

package validation

import (
	"fmt"
	"slices"
)

// TenantValidator provides common validation logic for tenant IDs across API versions.
type TenantValidator struct {
	// List of forbidden tenant ID values that pass the CRD pattern but are not allowed by Mimir
	ForbiddenValues []string
}

// NewTenantValidator creates a new TenantValidator with default forbidden values.
func NewTenantValidator() *TenantValidator {
	return &TenantValidator{
		ForbiddenValues: []string{"__mimir_cluster"},
	}
}

// ValidateTenantName validates a single tenant name for common business rules.
// This validates: forbidden values and basic naming rules.
// Duplicate checking is handled separately as it requires context of all tenants.
func (v *TenantValidator) ValidateTenantName(tenantName string) error {
	// Check forbidden values (CRD cannot enforce specific value exclusions)
	if slices.Contains(v.ForbiddenValues, tenantName) {
		return fmt.Errorf("tenant ID %q is not allowed. Forbidden values: %v", tenantName, v.ForbiddenValues)
	}

	return nil
}

// ValidateUniqueNames validates that all tenant names in the slice are unique.
func (v *TenantValidator) ValidateUniqueNames(tenantNames []string) error {
	// Track seen tenant IDs to detect duplicates (CRD can't enforce uniqueness in arrays)
	seen := make(map[string]bool)

	for _, tenantName := range tenantNames {
		// Check for duplicates (CRD cannot enforce this)
		if seen[tenantName] {
			return fmt.Errorf("duplicate tenant ID %q found", tenantName)
		}
		seen[tenantName] = true
	}

	return nil
}

// ValidateTenantNames validates a list of tenant names for all common rules.
// This combines forbidden value checking and duplicate detection.
func (v *TenantValidator) ValidateTenantNames(tenantNames []string) error {
	// First validate uniqueness
	if err := v.ValidateUniqueNames(tenantNames); err != nil {
		return err
	}

	// Then validate each name individually
	for _, tenantName := range tenantNames {
		if err := v.ValidateTenantName(tenantName); err != nil {
			return err
		}
	}

	return nil
}
