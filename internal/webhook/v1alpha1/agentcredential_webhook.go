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

package v1alpha1

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
)

var agentcredentiallog = logf.Log.WithName("agentcredential-resource")

// SetupAgentCredentialWebhookWithManager registers the webhook for AgentCredential.
func SetupAgentCredentialWebhookWithManager(mgr manager.Manager) error {
	err := ctrl.NewWebhookManagedBy(mgr, &observabilityv1alpha1.AgentCredential{}).
		WithValidator(&AgentCredentialValidator{Client: mgr.GetClient()}).
		WithValidatorCustomPath("/validate-v1alpha1-agent-credential").
		Complete()
	if err != nil {
		return fmt.Errorf("failed to build agentcredential webhook manager: %w", err)
	}
	return nil
}

// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// +kubebuilder:webhook:path=/validate-v1alpha1-agent-credential,mutating=false,failurePolicy=fail,sideEffects=None,groups=observability.giantswarm.io,resources=agentcredentials,verbs=create;update,versions=v1alpha1,name=agentcredentialv1alpha1.observability.giantswarm.io,admissionReviewVersions=v1

// AgentCredentialValidator validates AgentCredential resources on create/update.
//
// +kubebuilder:object:generate=false
type AgentCredentialValidator struct {
	Client client.Reader
}

var _ admission.Validator[*observabilityv1alpha1.AgentCredential] = &AgentCredentialValidator{}

// Scheme wires the validator to the manager. Not used: present for interface completeness.
func (v *AgentCredentialValidator) Scheme() *runtime.Scheme { return nil }

// ValidateCreate enforces business rules beyond what kubebuilder markers can express.
func (v *AgentCredentialValidator) ValidateCreate(ctx context.Context, obj *observabilityv1alpha1.AgentCredential) (admission.Warnings, error) {
	agentcredentiallog.Info("validating agent credential on create", "name", obj.GetName())
	if err := v.validateUnique(ctx, obj); err != nil {
		return nil, err
	}
	return nil, nil
}

// ValidateUpdate enforces immutability of spec fields and uniqueness.
func (v *AgentCredentialValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *observabilityv1alpha1.AgentCredential) (admission.Warnings, error) {
	agentcredentiallog.Info("validating agent credential on update", "name", newObj.GetName())

	var errs field.ErrorList
	if oldObj.Spec.Backend != newObj.Spec.Backend {
		errs = append(errs, field.Forbidden(field.NewPath("spec", "backend"), "backend is immutable"))
	}
	if oldObj.Spec.AgentName != newObj.Spec.AgentName {
		errs = append(errs, field.Forbidden(field.NewPath("spec", "agentName"), "agentName is immutable"))
	}
	if oldObj.Spec.SecretName != newObj.Spec.SecretName {
		errs = append(errs, field.Forbidden(field.NewPath("spec", "secretName"), "secretName is immutable"))
	}
	if len(errs) > 0 {
		return nil, apierrors.NewInvalid(newObj.GroupVersionKind().GroupKind(), newObj.Name, errs)
	}

	if err := v.validateUnique(ctx, newObj); err != nil {
		return nil, err
	}
	return nil, nil
}

// ValidateDelete is a no-op.
func (v *AgentCredentialValidator) ValidateDelete(ctx context.Context, obj *observabilityv1alpha1.AgentCredential) (admission.Warnings, error) {
	return nil, nil
}

// validateUnique rejects AgentCredentials with a (backend, agentName) pair that
// already exists elsewhere — two CRs producing the same htpasswd entry would
// conflict in the gateway aggregation.
func (v *AgentCredentialValidator) validateUnique(ctx context.Context, obj *observabilityv1alpha1.AgentCredential) error {
	if v.Client == nil {
		return nil
	}
	list := &observabilityv1alpha1.AgentCredentialList{}
	if err := v.Client.List(ctx, list); err != nil {
		return fmt.Errorf("failed to list agent credentials for uniqueness check: %w", err)
	}
	for i := range list.Items {
		other := &list.Items[i]
		if other.UID == obj.UID {
			continue
		}
		if other.Spec.Backend == obj.Spec.Backend && other.Spec.AgentName == obj.Spec.AgentName {
			return apierrors.NewInvalid(
				obj.GroupVersionKind().GroupKind(),
				obj.Name,
				field.ErrorList{field.Duplicate(
					field.NewPath("spec", "agentName"),
					fmt.Sprintf("agentName %q already used by %s/%s for backend %q", obj.Spec.AgentName, other.Namespace, other.Name, obj.Spec.Backend),
				)},
			)
		}
	}
	return nil
}
