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

package v1

import (
	"context"
	"errors"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/giantswarm/observability-operator/internal/predicates"
	"github.com/giantswarm/observability-operator/pkg/alertmanager"
	"github.com/giantswarm/observability-operator/pkg/common/tenancy"
)

// log is for logging in this package.
var log = logf.Log.WithName("alertmanagerconfig-secret-resource")

// SetupAlertmanagerConfigSecretWebhookWithManager registers the webhook for Secret in the manager.
func SetupAlertmanagerConfigSecretWebhookWithManager(mgr ctrl.Manager) error {
	err := ctrl.NewWebhookManagedBy(mgr).
		For(&corev1.Secret{}).
		WithValidator(&AlertmanagerConfigSecretValidator{
			client:           mgr.GetClient(),
			tenantRepository: tenancy.NewTenantRepository(mgr.GetClient()),
		}).
		WithCustomPath("/validate-alertmanager-config").
		Complete()
	if err != nil {
		return fmt.Errorf("failed to build alertmanager webhook manager: %w", err)
	}

	return nil
}

// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-alertmanager-config,mutating=false,failurePolicy=fail,sideEffects=None,groups="",resources=secrets,verbs=create;update,versions=v1,name=vsecret-v1.kb.io,admissionReviewVersions=v1

// AlertmanagerConfigSecretValidator struct is responsible for validating the Secret resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
// +kubebuilder:object:generate=false
type AlertmanagerConfigSecretValidator struct {
	client           client.Client
	tenantRepository tenancy.TenantRepository
}

var _ webhook.CustomValidator = &AlertmanagerConfigSecretValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Secret.
func (v *AlertmanagerConfigSecretValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil, fmt.Errorf("expected a Secret object but got %T", obj)
	}

	// Only validate secrets that are specifically marked as alertmanager-config
	if !v.isAlertmanagerConfigSecret(secret) {
		return nil, nil
	}

	log.Info("Validation for Secret upon creation", "name", secret.GetName())

	if err := v.validateTenant(ctx, secret); err != nil {
		return nil, err
	}
	return nil, validateAlertmanagerConfig(secret)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Secret.
func (v *AlertmanagerConfigSecretValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	secret, ok := newObj.(*corev1.Secret)
	if !ok {
		return nil, fmt.Errorf("expected a Secret object for the newObj but got %T", newObj)
	}

	// Only validate secrets that are specifically marked as alertmanager-config
	if !v.isAlertmanagerConfigSecret(secret) {
		return nil, nil
	}

	log.Info("Validation for Secret upon update", "name", secret.GetName())

	if err := v.validateTenant(ctx, secret); err != nil {
		return nil, err
	}
	return nil, validateAlertmanagerConfig(secret)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Secret.
func (v *AlertmanagerConfigSecretValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// We have nothing to validate on deletion
	return nil, nil
}

func (v *AlertmanagerConfigSecretValidator) validateTenant(ctx context.Context, secret *corev1.Secret) error {
	// Check that the secret has the correct labels.
	tenant, ok := secret.Labels[tenancy.TenantSelectorLabel]
	if !ok {
		return fmt.Errorf("%s label is required", tenancy.TenantSelectorLabel)
	}

	// Check that the tenant is defined in a Grafana Organization.
	tenants, err := v.tenantRepository.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tenants: %w", err)
	}
	if !slices.Contains(tenants, tenant) {
		return fmt.Errorf("tenant %q is not in the list of accepted tenants defined in GrafanaOrganizations", tenant)
	}

	// Check that there is only one alertmanager config for the tenant.
	var secretList corev1.SecretList
	err = v.client.List(ctx, &secretList, client.InNamespace(""),
		client.MatchingLabelsSelector{
			Selector: labels.SelectorFromSet(
				labels.Set{
					predicates.AlertmanagerConfigSelectorLabelName: predicates.AlertmanagerConfigSelectorLabelValue,
					tenancy.TenantSelectorLabel:                    tenant,
				},
			),
		})
	if err != nil {
		return fmt.Errorf("failed to list secrets for tenant %s: %w", tenant, err)
	}

	if len(secretList.Items) > 0 {
		for _, s := range secretList.Items {
			if s.Name != secret.Name || s.Namespace != secret.Namespace {
				err = errors.Join(err, fmt.Errorf("tenant %q already exists in secret %s/%s", tenant, s.Name, s.Namespace))
			}
		}
		return err
	}

	return nil
}

// isAlertmanagerConfigSecret checks if the secret is specifically marked as an alertmanager config secret
func (v *AlertmanagerConfigSecretValidator) isAlertmanagerConfigSecret(secret *corev1.Secret) bool {
	if secret.Labels == nil {
		return false
	}

	// Check for the specific label that identifies this as an alertmanager config secret
	kind, hasKindLabel := secret.Labels[predicates.AlertmanagerConfigSelectorLabelName]
	if !hasKindLabel || kind != predicates.AlertmanagerConfigSelectorLabelValue {
		return false
	}

	// Also check if it has a tenant label (required for alertmanager config secrets)
	_, hasTenantLabel := secret.Labels[tenancy.TenantSelectorLabel]
	return hasTenantLabel
}

func validateAlertmanagerConfig(secret *corev1.Secret) error {
	content, err := alertmanager.ExtractAlertmanagerConfig(secret)
	if err != nil {
		return fmt.Errorf("alertmanager configuration validation failed: %w. Note: If you're using a newer Alertmanager feature, it might not be supported yet by the Grafana fork (grafana/prometheus-alertmanager) used by Mimir", err)
	}

	_, err = alertmanager.ParseAlertmanagerConfig(content)
	if err != nil {
		return fmt.Errorf("alertmanager configuration validation failed: %w. Note: If you're using a newer Alertmanager feature, it might not be supported yet by the Grafana fork (grafana/prometheus-alertmanager) used by Mimir", err)
	}

	log.Info("alertmanager config validation successful")
	return nil
}
