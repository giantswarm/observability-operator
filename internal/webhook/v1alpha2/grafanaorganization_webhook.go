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

package v1alpha2

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	observabilityv1alpha2 "github.com/giantswarm/observability-operator/api/v1alpha2"
	webhookshared "github.com/giantswarm/observability-operator/internal/webhook"
)

// grafanaorganizationlog is for logging in this package.
var grafanaorganizationlog = logf.Log.WithName("grafanaorganization-v1alpha2-resource")

// SetupGrafanaOrganizationWebhookWithManager registers the webhook for GrafanaOrganization in the manager.
func SetupGrafanaOrganizationWebhookWithManager(mgr ctrl.Manager) error {
	err := ctrl.NewWebhookManagedBy(mgr).
		For(&observabilityv1alpha2.GrafanaOrganization{}).
		WithValidator(NewGrafanaOrganizationValidator()).
		WithCustomPath("/validate-v1alpha2-grafana-organization").
		Complete()
	if err != nil {
		return fmt.Errorf("failed to build v1alpha2 grafanaorganization webhook manager: %w", err)
	}

	return nil
}

// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-v1alpha2-grafana-organization,mutating=false,failurePolicy=fail,sideEffects=None,groups=observability.giantswarm.io,resources=grafanaorganizations,verbs=create;update,versions=v1alpha2,name=vgrafanaorganizationv1alpha2.kb.io,admissionReviewVersions=v1

// GrafanaOrganizationValidator struct is responsible for validating the GrafanaOrganization resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
//
// +kubebuilder:object:generate=false
type GrafanaOrganizationValidator struct {
	shared *webhookshared.GrafanaOrganizationValidator
}

// NewGrafanaOrganizationValidator creates a new validator instance
func NewGrafanaOrganizationValidator() *GrafanaOrganizationValidator {
	return &GrafanaOrganizationValidator{
		shared: &webhookshared.GrafanaOrganizationValidator{},
	}
}

var _ webhook.CustomValidator = &GrafanaOrganizationValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type GrafanaOrganization.
func (v *GrafanaOrganizationValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	grafanaorganization, ok := obj.(*observabilityv1alpha2.GrafanaOrganization)
	if !ok {
		return nil, fmt.Errorf("expected a v1alpha2 GrafanaOrganization object but got %T", obj)
	}

	grafanaorganizationlog.Info("Validation for v1alpha2 GrafanaOrganization upon creation", "name", grafanaorganization.GetName())

	return nil, v.shared.ValidateTenantConfigs(grafanaorganization.Spec.Tenants)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type GrafanaOrganization.
func (v *GrafanaOrganizationValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	grafanaorganization, ok := newObj.(*observabilityv1alpha2.GrafanaOrganization)
	if !ok {
		return nil, fmt.Errorf("expected a v1alpha2 GrafanaOrganization object for the newObj but got %T", newObj)
	}

	grafanaorganizationlog.Info("Validation for v1alpha2 GrafanaOrganization upon update", "name", grafanaorganization.GetName())

	return nil, v.shared.ValidateTenantConfigs(grafanaorganization.Spec.Tenants)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type GrafanaOrganization.
func (v *GrafanaOrganizationValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// We have nothing to validate on deletion
	return nil, nil
}


