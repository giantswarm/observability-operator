/*
Copyright 2026.

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
	"testing"
)

func TestNewTenantValidator(t *testing.T) {
	validator := NewTenantValidator()
	if validator == nil {
		t.Error("Expected NewTenantValidator to return a non-nil validator")
	}

	// Check default forbidden values are set
	if len(validator.ForbiddenValues) == 0 {
		t.Error("Expected default forbidden values to be set")
	}

	expectedForbidden := "__mimir_cluster"
	found := false
	for _, forbidden := range validator.ForbiddenValues {
		if forbidden == expectedForbidden {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected %q to be in forbidden values", expectedForbidden)
	}
}

func TestValidateTenantName(t *testing.T) {
	validator := NewTenantValidator()

	tests := []struct {
		name        string
		tenantName  string
		shouldError bool
		errorText   string
	}{
		{
			name:        "valid tenant name",
			tenantName:  "valid_tenant",
			shouldError: false,
		},
		{
			name:        "forbidden tenant name",
			tenantName:  "__mimir_cluster",
			shouldError: true,
			errorText:   "not allowed",
		},
		{
			name:        "another valid tenant name",
			tenantName:  "production_data",
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateTenantName(tt.tenantName)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error for tenant name %q, but got none", tt.tenantName)
				} else if tt.errorText != "" && !containsString(err.Error(), tt.errorText) {
					t.Errorf("Expected error to contain %q, got %q", tt.errorText, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for tenant name %q, got %v", tt.tenantName, err)
				}
			}
		})
	}
}

func TestValidateUniqueNames(t *testing.T) {
	validator := NewTenantValidator()

	tests := []struct {
		name        string
		tenantNames []string
		shouldError bool
		errorText   string
	}{
		{
			name:        "unique tenant names",
			tenantNames: []string{"tenant1", "tenant2", "tenant3"},
			shouldError: false,
		},
		{
			name:        "duplicate tenant names",
			tenantNames: []string{"tenant1", "tenant2", "tenant1"},
			shouldError: true,
			errorText:   "duplicate tenant ID",
		},
		{
			name:        "empty list",
			tenantNames: []string{},
			shouldError: false,
		},
		{
			name:        "single tenant",
			tenantNames: []string{"single_tenant"},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateUniqueNames(tt.tenantNames)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error for tenant names %v, but got none", tt.tenantNames)
				} else if tt.errorText != "" && !containsString(err.Error(), tt.errorText) {
					t.Errorf("Expected error to contain %q, got %q", tt.errorText, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for tenant names %v, got %v", tt.tenantNames, err)
				}
			}
		})
	}
}

func TestValidateTenantNames(t *testing.T) {
	validator := NewTenantValidator()

	tests := []struct {
		name        string
		tenantNames []string
		shouldError bool
		errorText   string
	}{
		{
			name:        "valid unique tenant names",
			tenantNames: []string{"tenant1", "tenant2", "tenant3"},
			shouldError: false,
		},
		{
			name:        "duplicate tenant names",
			tenantNames: []string{"tenant1", "tenant2", "tenant1"},
			shouldError: true,
			errorText:   "duplicate tenant ID",
		},
		{
			name:        "forbidden tenant name",
			tenantNames: []string{"tenant1", "__mimir_cluster"},
			shouldError: true,
			errorText:   "not allowed",
		},
		{
			name:        "both forbidden and duplicate",
			tenantNames: []string{"__mimir_cluster", "__mimir_cluster"},
			shouldError: true,
			errorText:   "duplicate tenant ID", // Should catch duplicate first
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateTenantNames(tt.tenantNames)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error for tenant names %v, but got none", tt.tenantNames)
				} else if tt.errorText != "" && !containsString(err.Error(), tt.errorText) {
					t.Errorf("Expected error to contain %q, got %q", tt.errorText, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for tenant names %v, got %v", tt.tenantNames, err)
				}
			}
		})
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
