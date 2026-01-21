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

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/giantswarm/observability-operator/internal/mapper"
)

// dashboardconfigmaplog is for logging in this package.
var dashboardconfigmaplog = logf.Log.WithName("dashboardconfigmap-resource")

// SetupDashboardConfigMapWebhookWithManager registers the webhook for ConfigMap in the manager.
func SetupDashboardConfigMapWebhookWithManager(mgr manager.Manager) error {
	err := ctrl.NewWebhookManagedBy(mgr, &corev1.ConfigMap{}).
		WithValidator(&DashboardConfigMapValidator{
			client:          mgr.GetClient(),
			dashboardMapper: mapper.New(),
		}).
		WithValidatorCustomPath("/validate-dashboard-configmap").
		Complete()
	if err != nil {
		return fmt.Errorf("failed to build dashboard webhook manager: %w", err)
	}

	return nil
}

// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-dashboard-configmap,mutating=false,failurePolicy=fail,sideEffects=None,groups="",resources=configmaps,verbs=create;update,versions=v1,name=dashboardconfigmap.observability.giantswarm.io,admissionReviewVersions=v1

// DashboardConfigMapValidator struct is responsible for validating the ConfigMap resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
//
// +kubebuilder:object:generate=false
type DashboardConfigMapValidator struct {
	client          client.Client
	dashboardMapper *mapper.DashboardMapper
}

var _ admission.Validator[*corev1.ConfigMap] = &DashboardConfigMapValidator{}

// ValidateCreate implements admission.Validator so a webhook will be registered for the type ConfigMap.
func (v *DashboardConfigMapValidator) ValidateCreate(ctx context.Context, Obj *corev1.ConfigMap) (admission.Warnings, error) {
	// Only validate ConfigMaps that are specifically marked as dashboard ConfigMaps
	if !v.isDashboardConfigMap(Obj) {
		return nil, nil
	}

	dashboardconfigmaplog.Info("Validation for dashboard ConfigMap upon creation", "name", Obj.GetName())

	return v.validateDashboard(Obj)
}

// ValidateUpdate implements admission.Validator so a webhook will be registered for the type ConfigMap.
func (v *DashboardConfigMapValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *corev1.ConfigMap) (admission.Warnings, error) {
	// Only validate ConfigMaps that are specifically marked as dashboard ConfigMaps
	if !v.isDashboardConfigMap(newObj) {
		return nil, nil
	}

	dashboardconfigmaplog.Info("Validation for dashboard ConfigMap upon update", "name", newObj.GetName())
	return v.validateDashboard(newObj)
}

// ValidateDelete implements admission.Validator so a webhook will be registered for the type ConfigMap.
func (v *DashboardConfigMapValidator) ValidateDelete(ctx context.Context, obj *corev1.ConfigMap) (admission.Warnings, error) {
	// We have nothing to validate on deletion
	return nil, nil
}

// validateDashboard validates a dashboard ConfigMap using domain validation logic
func (v *DashboardConfigMapValidator) validateDashboard(configmap *corev1.ConfigMap) (admission.Warnings, error) {
	// Convert ConfigMap to domain objects using mapper
	dashboards := v.dashboardMapper.FromConfigMap(configmap)

	// Validate each dashboard using domain validation directly
	for _, dash := range dashboards {
		errs := dash.Validate()
		if len(errs) > 0 {
			return nil, fmt.Errorf("dashboard validation failed for uid %s: %w", dash.UID(), errors.Join(errs...))
		}
	}

	return nil, nil
}

// isDashboardConfigMap checks if the ConfigMap is specifically marked as a dashboard ConfigMap
func (v *DashboardConfigMapValidator) isDashboardConfigMap(configmap *corev1.ConfigMap) bool {
	if configmap.Labels == nil {
		return false
	}

	// Check for the specific label that identifies this as a dashboard ConfigMap
	kind, hasKindLabel := configmap.Labels["app.giantswarm.io/kind"]
	if !hasKindLabel || kind != "dashboard" {
		return false
	}

	return true
}
