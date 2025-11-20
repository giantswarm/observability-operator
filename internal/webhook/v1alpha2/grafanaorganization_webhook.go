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
	"github.com/giantswarm/observability-operator/internal/webhook/validation"
)

// grafanaorganizationlog is for logging in this package.
var grafanaorganizationlog = logf.Log.WithName("grafanaorganization-v1alpha2-resource")

// SetupGrafanaOrganizationWebhookWithManager registers the webhook for GrafanaOrganization in the manager.
func SetupGrafanaOrganizationWebhookWithManager(mgr ctrl.Manager) error {
	err := ctrl.NewWebhookManagedBy(mgr).
		For(&observabilityv1alpha2.GrafanaOrganization{}).
		WithValidator(&GrafanaOrganizationValidator{}).
		WithCustomPath("/validate-v1alpha2-grafana-organization").
		Complete()
	if err != nil {
		return fmt.Errorf("failed to build grafanaorganization v1alpha2 webhook manager: %w", err)
	}

	return nil
}

// +kubebuilder:webhook:path=/validate-v1alpha2-grafana-organization,mutating=false,failurePolicy=fail,sideEffects=None,groups=observability.giantswarm.io,resources=grafanaorganizations,verbs=create;update,versions=v1alpha2,name=grafanaorganizationv1alpha2.observability.giantswarm.io,admissionReviewVersions=v1

// GrafanaOrganizationValidator struct is responsible for validating the GrafanaOrganization resource
// when it is created, updated, or deleted.
//
// +kubebuilder:object:generate=false
type GrafanaOrganizationValidator struct{}

var _ webhook.CustomValidator = &GrafanaOrganizationValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type GrafanaOrganization.
func (v *GrafanaOrganizationValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	grafanaorganization, ok := obj.(*observabilityv1alpha2.GrafanaOrganization)
	if !ok {
		return nil, fmt.Errorf("expected a GrafanaOrganization object but got %T", obj)
	}

	grafanaorganizationlog.Info("Validation for GrafanaOrganization v1alpha2 upon creation", "name", grafanaorganization.GetName())

	return nil, v.validateTenantConfigs(grafanaorganization.Spec.Tenants)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type GrafanaOrganization.
func (v *GrafanaOrganizationValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	grafanaorganization, ok := newObj.(*observabilityv1alpha2.GrafanaOrganization)
	if !ok {
		return nil, fmt.Errorf("expected a GrafanaOrganization object for the newObj but got %T", newObj)
	}

	grafanaorganizationlog.Info("Validation for GrafanaOrganization v1alpha2 upon update", "name", grafanaorganization.GetName())

	return nil, v.validateTenantConfigs(grafanaorganization.Spec.Tenants)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type GrafanaOrganization.
func (v *GrafanaOrganizationValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// We have nothing to validate on deletion
	return nil, nil
}

// validateTenantConfigs validates tenant configurations for business logic that cannot be expressed in OpenAPI schema.
func (v *GrafanaOrganizationValidator) validateTenantConfigs(tenantConfigs []observabilityv1alpha2.TenantConfig) error {
	// Convert TenantConfig slice to string slice for validation
	tenantNames := make([]string, len(tenantConfigs))
	for i, tenantConfig := range tenantConfigs {
		tenantNames[i] = string(tenantConfig.Name)
	}

	// Use shared validation logic for basic tenant name validation
	validator := validation.NewTenantValidator()
	if err := validator.ValidateTenantNames(tenantNames); err != nil {
		return err
	}

	return nil
}
