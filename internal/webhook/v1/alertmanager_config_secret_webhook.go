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
	"errors"
	"fmt"
	"slices"

	"github.com/prometheus/alertmanager/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/giantswarm/observability-operator/internal/predicates"
	"github.com/giantswarm/observability-operator/pkg/common/tenancy"
)

// log is for logging in this package.
var log = logf.Log.WithName("alertmanager-config-secrets-resource")

// SetupAlertmanagerConfigSecretWebhookWithManager registers the webhook for Secret in the manager.
func SetupAlertmanagerConfigSecretWebhookWithManager(mgr ctrl.Manager) (err error) {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&corev1.Secret{}).
		WithValidator(&AlertmanagerConfigSecretCustomValidator{
			client: mgr.GetClient(),
		}).
		WithCustomPath("/validate-alertmanager-config").
		Complete()
}

// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-alertmanager-config,mutating=false,failurePolicy=fail,sideEffects=None,groups="",resources=secrets,verbs=create;update,versions=v1,name=vsecret-v1.kb.io,admissionReviewVersions=v1

// SecretCustomValidator struct is responsible for validating the Secret resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.

type AlertmanagerConfigSecretCustomValidator struct {
	client client.Client
}

var _ webhook.CustomValidator = &AlertmanagerConfigSecretCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Secret.
func (v *AlertmanagerConfigSecretCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil, fmt.Errorf("expected a Secret object but got %T", obj)
	}

	log.Info("Validation for Secret upon creation", "name", secret.GetName())

	if err := v.validateTenant(ctx, secret); err != nil {
		return nil, err
	}
	return nil, validateAlertmanagerConfig(ctx, secret)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Secret.
func (v *AlertmanagerConfigSecretCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	secret, ok := newObj.(*corev1.Secret)
	if !ok {
		return nil, fmt.Errorf("expected a Secret object for the newObj but got %T", newObj)
	}

	log.Info("Validation for Secret upon update", "name", secret.GetName())

	if err := v.validateTenant(ctx, secret); err != nil {
		return nil, err
	}
	return nil, validateAlertmanagerConfig(ctx, secret)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Secret.
func (v *AlertmanagerConfigSecretCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// We have nothing to validate on deletion
	return nil, nil
}

func (v *AlertmanagerConfigSecretCustomValidator) validateTenant(ctx context.Context, secret *corev1.Secret) error {
	// Check that the secret has the correct labels.
	tenant, ok := secret.Labels[tenancy.TenantSelectorLabel]
	if !ok {
		return fmt.Errorf("%s label is required", tenancy.TenantSelectorLabel)
	}

	// Check that the tenant is defined in a Grafana Organization.
	tenants, err := tenancy.ListTenants(ctx, v.client)
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

func validateAlertmanagerConfig(ctx context.Context, secret *corev1.Secret) error {
	// Check that the secret contains an "alertmanager.yaml" file.
	alertmanagerConfig, found := secret.Data["alertmanager.yaml"]
	if !found {
		return fmt.Errorf("missing alertmanager.yaml in the secret")
	}
	_, err := config.Load(string(alertmanagerConfig))
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	// TODO add more validation on the templates directly
	log.Info("alertmanager config validation successful")
	return nil
}
