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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// dashboardconfigmaplog is for logging in this package.
var dashboardconfigmaplog = logf.Log.WithName("dashboardconfigmap-resource")

// SetupDashboardConfigMapWebhookWithManager registers the webhook for ConfigMap in the manager.
func SetupDashboardConfigMapWebhookWithManager(mgr ctrl.Manager) error {
	err := ctrl.NewWebhookManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		WithValidator(&DashboardConfigMapValidator{client: mgr.GetClient()}).
		WithCustomPath("/validate-dashboard-configmap").
		Complete()
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	return nil
}

// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-dashboard-configmap,mutating=false,failurePolicy=fail,sideEffects=None,groups="",resources=configmaps,verbs=create;update,versions=v1,name=vdashboardconfigmap.kb.io,admissionReviewVersions=v1

// DashboardConfigMapValidator struct is responsible for validating the ConfigMap resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
//
// +kubebuilder:object:generate=false
type DashboardConfigMapValidator struct {
	client client.Client
}

var _ webhook.CustomValidator = &DashboardConfigMapValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type ConfigMap.
func (v *DashboardConfigMapValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	configmap, ok := obj.(*corev1.ConfigMap)
	if !ok {
		return nil, fmt.Errorf("expected a ConfigMap object but got %T", obj)
	}

	// Only validate ConfigMaps that are specifically marked as dashboard ConfigMaps
	if !v.isDashboardConfigMap(configmap) {
		return nil, nil
	}

	dashboardconfigmaplog.Info("Validation for dashboard ConfigMap upon creation", "name", configmap.GetName())

	// TODO: Add dashboard validation logic here
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type ConfigMap.
func (v *DashboardConfigMapValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	configmap, ok := newObj.(*corev1.ConfigMap)
	if !ok {
		return nil, fmt.Errorf("expected a ConfigMap object for the newObj but got %T", newObj)
	}

	// Only validate ConfigMaps that are specifically marked as dashboard ConfigMaps
	if !v.isDashboardConfigMap(configmap) {
		return nil, nil
	}

	dashboardconfigmaplog.Info("Validation for dashboard ConfigMap upon update", "name", configmap.GetName())

	// TODO: Add dashboard validation logic here
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type ConfigMap.
func (v *DashboardConfigMapValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// We have nothing to validate on deletion
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
