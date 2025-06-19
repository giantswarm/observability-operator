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

package v1

import (
	"context"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
)

// grafanaorganizationlog is for logging in this package.
var grafanaorganizationlog = logf.Log.WithName("grafanaorganization-resource")

// SetupGrafanaOrganizationWebhookWithManager registers the webhook for GrafanaOrganization in the manager.
func SetupGrafanaOrganizationWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&observabilityv1alpha1.GrafanaOrganization{}).
		WithValidator(&GrafanaOrganizationValidator{}).
		WithCustomPath("/validate-v1alpha1-grafana-organization").
		Complete()
}

// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-v1alpha1-grafana-organization,mutating=false,failurePolicy=fail,sideEffects=None,groups=observability.giantswarm.io,resources=grafanaorganizations,verbs=create;update,versions=v1alpha1,name=vgrafanaorganization.kb.io,admissionReviewVersions=v1

// GrafanaOrganizationValidator struct is responsible for validating the GrafanaOrganization resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
//
// +kubebuilder:object:generate=false
type GrafanaOrganizationValidator struct{}

var _ webhook.CustomValidator = &GrafanaOrganizationValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type GrafanaOrganization.
func (v *GrafanaOrganizationValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	grafanaorganization, ok := obj.(*observabilityv1alpha1.GrafanaOrganization)
	if !ok {
		return nil, fmt.Errorf("expected a GrafanaOrganization object but got %T", obj)
	}

	grafanaorganizationlog.Info("Validation for GrafanaOrganization upon creation", "name", grafanaorganization.GetName())

	return nil, v.validateTenantIDs(grafanaorganization.Spec.Tenants)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type GrafanaOrganization.
func (v *GrafanaOrganizationValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	grafanaorganization, ok := newObj.(*observabilityv1alpha1.GrafanaOrganization)
	if !ok {
		return nil, fmt.Errorf("expected a GrafanaOrganization object for the newObj but got %T", newObj)
	}

	grafanaorganizationlog.Info("Validation for GrafanaOrganization upon update", "name", grafanaorganization.GetName())

	return nil, v.validateTenantIDs(grafanaorganization.Spec.Tenants)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type GrafanaOrganization.
func (v *GrafanaOrganizationValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// We have nothing to validate on deletion
	return nil, nil
}

// validateTenantIDs validates tenant IDs for business logic that cannot be expressed in OpenAPI schema.
// The CRD already validates: pattern (Alloy-compatible), minLength(1), maxLength(150), and minItems(1).
// This webhook only adds: forbidden values and duplicates validation.
// See: https://grafana.com/docs/mimir/latest/configure/about-tenant-ids/
func (v *GrafanaOrganizationValidator) validateTenantIDs(tenantIDs []observabilityv1alpha1.TenantID) error {
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
