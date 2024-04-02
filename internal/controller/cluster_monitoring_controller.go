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

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/giantswarm/observability-operator/pkg/common"
	"github.com/giantswarm/observability-operator/pkg/monitoring"
	"github.com/giantswarm/observability-operator/pkg/monitoring/heartbeat"
)

// ClusterMonitoringReconciler reconciles a Cluster object
type ClusterMonitoringReconciler struct {
	// Client is the controller client.
	client.Client
	common.ManagementCluster
	// HeartbeatRepository is the repository for managing heartbeats.
	heartbeat.HeartbeatRepository
	// MonitoringEnabled defines whether monitoring is enabled at the installation level.
	MonitoringEnabled bool
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterMonitoringReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1.Cluster{}).
		Complete(r)
}

//+kubebuilder:rbac:groups=objectstorage.giantswarm.io,resources=Clusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=objectstorage.giantswarm.io,resources=Clusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=objectstorage.giantswarm.io,resources=Clusters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *ClusterMonitoringReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Cluster instance.
	cluster := &clusterv1.Cluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return ctrl.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return ctrl.Result{}, errors.WithStack(err)
	}

	// Handle deletion reconciliation loop.
	if !cluster.ObjectMeta.DeletionTimestamp.IsZero() || !r.MonitoringEnabled {
		logger.Info("Handling deletion for Cluster", "cluster", cluster.Name)
		return r.reconcileDelete(ctx, cluster)
	}

	logger.Info("Reconciling Cluster", "cluster", cluster.Name)
	// Handle normal reconciliation loop.
	return r.reconcile(ctx, cluster)
}

// reconcile handles cluster reconciliation.
func (r *ClusterMonitoringReconciler) reconcile(ctx context.Context, cluster *clusterv1.Cluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("cluster", cluster.Name)

	// Add finalizer first if not set to avoid the race condition between init and delete.
	// Note: Finalizers in general can only be added when the deletionTimestamp is not set.
	if !controllerutil.ContainsFinalizer(cluster, monitoring.MonitoringFinalizer) {
		logger.Info("adding finalizer", "finalizer", monitoring.MonitoringFinalizer)
		controllerutil.AddFinalizer(cluster, monitoring.MonitoringFinalizer)
		err := r.Client.Update(ctx, cluster)
		if err != nil {
			logger.Error(err, "failed to add finalizer", "finalizer", monitoring.MonitoringFinalizer)
			return ctrl.Result{}, errors.WithStack(err)
		}
		logger.Info("added finalizer", "finalizer", monitoring.MonitoringFinalizer)
		return ctrl.Result{}, nil
	}

	if cluster.Name == r.ManagementCluster.Name {
		err := r.HeartbeatRepository.CreateOrUpdate(ctx)
		if err != nil {
			logger.Error(err, "failed to create or update heartbeat")
			return ctrl.Result{Requeue: true}, errors.WithStack(err)
		}
	}

	return ctrl.Result{}, nil
}

// reconcileDelete handles cluster deletion.
func (r *ClusterMonitoringReconciler) reconcileDelete(ctx context.Context, cluster *clusterv1.Cluster) (reconcile.Result, error) {
	logger := log.FromContext(ctx).WithValues("cluster", cluster.Name)
	if controllerutil.ContainsFinalizer(cluster, monitoring.MonitoringFinalizer) {

		if cluster.Name == r.ManagementCluster.Name {
			err := r.HeartbeatRepository.Delete(ctx)
			if err != nil {
				logger.Error(err, "failed to delete heartbeat")
				return ctrl.Result{Requeue: true}, errors.WithStack(err)
			}
		}

		// We get the latest state of the object to avoid race conditions.
		// Finalizer handling needs to come last.
		logger.Info("removing finalizer", "finalizer", monitoring.MonitoringFinalizer)
		controllerutil.RemoveFinalizer(cluster, monitoring.MonitoringFinalizer)
		err := r.Client.Update(ctx, cluster)
		if err != nil {
			// We need to requeue if we fail to remove the finalizer because of race conditions between multiple operators.
			// This will be eventually consistent.
			logger.Error(err, "failed to remove finalizer, requeuing", "finalizer", monitoring.MonitoringFinalizer)
			return ctrl.Result{Requeue: true}, nil
		}
		logger.Info("removed finalizer", "finalizer", monitoring.MonitoringFinalizer)
	}
	controllerutil.RemoveFinalizer(cluster, monitoring.MonitoringFinalizer)
	return ctrl.Result{}, nil
}
