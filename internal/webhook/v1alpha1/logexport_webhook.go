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
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/giantswarm/observability-operator/internal/webhook/validation"
)

var logexportlog = logf.Log.WithName("logexport-resource")

// SetupLogExportWebhookWithManager registers the webhook for LogExport.
func SetupLogExportWebhookWithManager(mgr manager.Manager) error {
	err := ctrl.NewWebhookManagedBy(mgr, &observabilityv1alpha1.LogExport{}).
		WithValidator(&LogExportValidator{}).
		WithValidatorCustomPath("/validate-v1alpha1-log-export").
		Complete()
	if err != nil {
		return fmt.Errorf("failed to build logexport webhook manager: %w", err)
	}
	return nil
}

// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// +kubebuilder:webhook:path=/validate-v1alpha1-log-export,mutating=false,failurePolicy=fail,sideEffects=None,groups=observability.giantswarm.io,resources=logexports,verbs=create;update,versions=v1alpha1,name=logexportv1alpha1.observability.giantswarm.io,admissionReviewVersions=v1

// LogExportValidator validates LogExport resources on create/update.
//
// Rejecting a bad selector here rather than at reconcile time is deliberate: the
// exporter is shared by the whole installation and exits on a bad config, so one
// customer's mistake would otherwise stop every export on it.
//
// +kubebuilder:object:generate=false
type LogExportValidator struct{}

var _ admission.Validator[*observabilityv1alpha1.LogExport] = &LogExportValidator{}

// ValidateCreate enforces business rules beyond what kubebuilder markers can express.
func (v *LogExportValidator) ValidateCreate(ctx context.Context, obj *observabilityv1alpha1.LogExport) (admission.Warnings, error) {
	logexportlog.Info("validating log export on create", "name", obj.GetName())
	return nil, validateSpec(obj)
}

// ValidateUpdate applies the same rules as create.
func (v *LogExportValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *observabilityv1alpha1.LogExport) (admission.Warnings, error) {
	logexportlog.Info("validating log export on update", "name", newObj.GetName())
	return nil, validateSpec(newObj)
}

// ValidateDelete is a no-op.
func (v *LogExportValidator) ValidateDelete(ctx context.Context, obj *observabilityv1alpha1.LogExport) (admission.Warnings, error) {
	return nil, nil
}

func validateSpec(obj *observabilityv1alpha1.LogExport) error {
	var errs field.ErrorList
	if err := validation.ValidateSelector(obj.Spec.Selector); err != nil {
		errs = append(errs, field.Invalid(field.NewPath("spec", "selector"), obj.Spec.Selector, err.Error()))
	}
	if len(errs) > 0 {
		return apierrors.NewInvalid(obj.GroupVersionKind().GroupKind(), obj.Name, errs)
	}
	return nil
}
