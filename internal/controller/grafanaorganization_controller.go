/*
Copyright 2024.

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

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
)

// GrafanaOrganizationReconciler reconciles a GrafanaOrganization object
type GrafanaOrganizationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=observability.giantswarm.io,resources=grafanaorganizations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=observability.giantswarm.io,resources=grafanaorganizations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=observability.giantswarm.io,resources=grafanaorganizations/finalizers,verbs=update

// Reconcile is part of the main Kubernetes reconciliation loop which aims to
// move the current state of the GrafanaOrganization closer to the desired state.
// TODO(zirko): Modify the Reconcile function to compare the state specified by
// the GrafanaOrganization object against the actual organization state, and then
// perform operations to make the organization state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.0/pkg/reconcile
func (r *GrafanaOrganizationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(zirko): your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GrafanaOrganizationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&observabilityv1alpha1.GrafanaOrganization{}).
		Complete(r)
}
