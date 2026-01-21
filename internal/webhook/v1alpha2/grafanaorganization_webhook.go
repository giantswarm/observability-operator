package v1alpha2

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	observabilityv1alpha2 "github.com/giantswarm/observability-operator/api/v1alpha2"
	"github.com/giantswarm/observability-operator/internal/webhook/validation"
)

// grafanaorganizationlog is for logging in this package.
var grafanaorganizationlog = logf.Log.WithName("grafanaorganization-v1alpha2-resource")

// SetupGrafanaOrganizationWebhookWithManager registers the webhook for GrafanaOrganization in the manager.
func SetupGrafanaOrganizationWebhookWithManager(mgr manager.Manager) error {
	err := ctrl.NewWebhookManagedBy(mgr, &observabilityv1alpha2.GrafanaOrganization{}).
		WithValidator(&GrafanaOrganizationValidator{}).
		WithValidatorCustomPath("/validate-v1alpha2-grafana-organization").
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

var _ admission.Validator[*observabilityv1alpha2.GrafanaOrganization] = &GrafanaOrganizationValidator{}

// ValidateCreate implements admission.Validator so a webhook will be registered for the type GrafanaOrganization.
func (v *GrafanaOrganizationValidator) ValidateCreate(ctx context.Context, obj *observabilityv1alpha2.GrafanaOrganization) (admission.Warnings, error) {
	grafanaorganizationlog.Info("Validation for GrafanaOrganization v1alpha2 upon creation", "name", obj.GetName())

	return nil, v.validateTenantConfigs(obj.Spec.Tenants)
}

// ValidateUpdate implements admission.Validator so a webhook will be registered for the type GrafanaOrganization.
func (v *GrafanaOrganizationValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *observabilityv1alpha2.GrafanaOrganization) (admission.Warnings, error) {
	grafanaorganizationlog.Info("Validation for GrafanaOrganization v1alpha2 upon update", "name", newObj.GetName())

	return nil, v.validateTenantConfigs(newObj.Spec.Tenants)
}

// ValidateDelete implements admission.Validator so a webhook will be registered for the type GrafanaOrganization.
func (v *GrafanaOrganizationValidator) ValidateDelete(ctx context.Context, obj *observabilityv1alpha2.GrafanaOrganization) (admission.Warnings, error) {
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
