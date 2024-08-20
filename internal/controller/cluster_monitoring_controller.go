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
	"time"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/giantswarm/observability-operator/pkg/bundle"
	"github.com/giantswarm/observability-operator/pkg/common"
	commonmonitoring "github.com/giantswarm/observability-operator/pkg/common/monitoring"
	"github.com/giantswarm/observability-operator/pkg/monitoring"
	"github.com/giantswarm/observability-operator/pkg/monitoring/alloy"
	"github.com/giantswarm/observability-operator/pkg/monitoring/heartbeat"
	"github.com/giantswarm/observability-operator/pkg/monitoring/mimir"
	"github.com/giantswarm/observability-operator/pkg/monitoring/prometheusagent"
)

var (
	observabilityBundleVersionSupportAlloyMetrics = semver.MustParse("1.6.0")
)

// ClusterMonitoringReconciler reconciles a Cluster object
type ClusterMonitoringReconciler struct {
	// Client is the controller client.
	client.Client
	common.ManagementCluster
	// PrometheusAgentService is the service for managing PrometheusAgent resources.
	prometheusagent.PrometheusAgentService
	// AlloyService is the service which manages Alloy monitoring agent configuration.
	AlloyService alloy.Service
	// HeartbeatRepository is the repository for managing heartbeats.
	heartbeat.HeartbeatRepository
	// MimirService is the service for managing mimir configuration.
	mimir.MimirService
	// BundleConfigurationService is the service for configuring the observability bundle.
	*bundle.BundleConfigurationService
	// MonitoringConfig is the configuration for the monitoring package.
	MonitoringConfig monitoring.Config
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

	// Linting is disabled for the following line as otherwise it fails with the following error:
	// "should not use built-in type string as key for value"
	logger := log.FromContext(ctx).WithValues("installation", r.ManagementCluster.Name) // nolint
	ctx = log.IntoContext(ctx, logger)

	if !r.MonitoringConfig.Enabled {
		logger.Info("monitoring is disabled at the installation level.")
	}

	if !r.MonitoringConfig.IsMonitored(cluster) {
		logger.Info("monitoring is disabled for this cluster.")
	}

	// Handle deletion reconciliation loop.
	if !cluster.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("handling deletion for cluster")
		return r.reconcileDelete(ctx, cluster)
	}

	logger.Info("reconciling cluster")
	// Handle normal reconciliation loop.
	return r.reconcile(ctx, cluster)
}

// reconcile handles cluster reconciliation.
func (r *ClusterMonitoringReconciler) reconcile(ctx context.Context, cluster *clusterv1.Cluster) (ctrl.Result, error) {
	var err error
	logger := log.FromContext(ctx)

	// Add finalizer first if not set to avoid the race condition between init and delete.
	// Note: Finalizers in general can only be added when the deletionTimestamp is not set.
	if !controllerutil.ContainsFinalizer(cluster, monitoring.MonitoringFinalizer) {
		// We use a patch rather than an update to avoid conflicts when multiple controllers are adding their finalizer to the ClusterCR
		// We use the patch from sigs.k8s.io/cluster-api/util/patch to handle the patching without conflicts
		logger.Info("adding finalizer", "finalizer", monitoring.MonitoringFinalizer)
		patchHelper, err := patch.NewHelper(cluster, r.Client)
		if err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
		controllerutil.AddFinalizer(cluster, monitoring.MonitoringFinalizer)
		if err := patchHelper.Patch(ctx, cluster); err != nil {
			logger.Error(err, "failed to add finalizer", "finalizer", monitoring.MonitoringFinalizer)
			return ctrl.Result{}, errors.WithStack(err)
		}
		logger.Info("added finalizer", "finalizer", monitoring.MonitoringFinalizer)
		return ctrl.Result{}, nil
	}

	// Management cluster specific configuration
	if cluster.Name == r.ManagementCluster.Name {
		// If monitoring is enabled as the installation level, configure the monitoring stack, otherwise, tear it down.
		if r.MonitoringConfig.Enabled {
			err = r.HeartbeatRepository.CreateOrUpdate(ctx)
			if err != nil {
				logger.Error(err, "failed to create or update heartbeat")
				return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
			}

			err = r.MimirService.ConfigureMimir(ctx)
			if err != nil {
				logger.Error(err, "failed to configure mimir")
				return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
			}
		} else {
			err = r.tearDown(ctx)
			if err != nil {
				logger.Error(err, "failed to tear down the monitoring stack")
				return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
			}
		}
	}

	// Enforce prometheus-agent as monitoring agent when observability-bundle version < 1.6.0
	monitoringAgent := r.MonitoringConfig.MonitoringAgent
	observabilityBundleVersion, err := commonmonitoring.GetObservabilityBundleAppVersion(cluster, r.Client, ctx)
	if err != nil {
		logger.Error(err, "failed to configure get observability-bundle version")
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
	}
	if observabilityBundleVersion.LT(observabilityBundleVersionSupportAlloyMetrics) && monitoringAgent != commonmonitoring.MonitoringAgentPrometheus {
		logger.Info("Monitoring agent is not supported by observability bundle, using prometheus-agent instead.", "observability-bundle-version", observabilityBundleVersion, "monitoring-agent", monitoringAgent)
		monitoringAgent = commonmonitoring.MonitoringAgentPrometheus
	}
	r.MonitoringConfig.MonitoringAgent = monitoringAgent
	r.BundleConfigurationService.SetMonitoringAgent(monitoringAgent)
	r.AlloyService.SetMonitoringAgent(monitoringAgent)

	// We always configure the bundle, even if monitoring is disabled for the cluster.
	err = r.BundleConfigurationService.Configure(ctx, cluster)
	if err != nil {
		logger.Error(err, "failed to configure the observability-bundle")
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	// Cluster specific configuration
	if r.MonitoringConfig.IsMonitored(cluster) {
		switch r.MonitoringConfig.MonitoringAgent {
		case commonmonitoring.MonitoringAgentPrometheus:
			// Create or update PrometheusAgent remote write configuration.
			err = r.PrometheusAgentService.ReconcileRemoteWriteConfiguration(ctx, cluster)
			if err != nil {
				logger.Error(err, "failed to create or update prometheus agent remote write config")
				return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
			}
		case commonmonitoring.MonitoringAgentAlloy:
			// Create or update Alloy monitoring configuration.
			err = r.AlloyService.ReconcileCreate(ctx, cluster)
			if err != nil {
				logger.Error(err, "failed to create or update alloy monitoring config")
				return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
			}
		default:
			return ctrl.Result{}, errors.Errorf("unsupported monitoring agent %q", r.MonitoringConfig.MonitoringAgent)
		}
	} else {
		// clean up any existing prometheus agent configuration
		err := r.PrometheusAgentService.DeleteRemoteWriteConfiguration(ctx, cluster)
		if err != nil {
			logger.Error(err, "failed to delete prometheus agent remote write config")
			return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
		}

		// clean up any existing alloy monitoring configuration
		err = r.AlloyService.ReconcileDelete(ctx, cluster)
		if err != nil {
			logger.Error(err, "failed to delete alloy monitoring config")
			return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
		}
	}

	return ctrl.Result{}, nil
}

// reconcileDelete handles cluster deletion.
func (r *ClusterMonitoringReconciler) reconcileDelete(ctx context.Context, cluster *clusterv1.Cluster) (reconcile.Result, error) {
	logger := log.FromContext(ctx)

	// We do not need to delete anything if there is no finalizer on the cluster
	if controllerutil.ContainsFinalizer(cluster, monitoring.MonitoringFinalizer) {
		// We always remove the bundle configure, even if monitoring is disabled for the cluster.
		err := r.BundleConfigurationService.RemoveConfiguration(ctx, cluster)
		if err != nil {
			logger.Error(err, "failed to remove the observability-bundle configuration")
			return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
		}

		// Cluster specific configuration
		if r.MonitoringConfig.IsMonitored(cluster) {
			err := r.PrometheusAgentService.DeleteRemoteWriteConfiguration(ctx, cluster)
			if err != nil {
				logger.Error(err, "failed to delete prometheus agent remote write config")
				return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
			}
		}

		// Management cluster specific configuration
		if cluster.Name == r.ManagementCluster.Name {
			err := r.tearDown(ctx)
			if err != nil {
				logger.Error(err, "failed to tear down the monitoring stack")
				return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
			}
		}

		// We get the latest state of the object to avoid race conditions.
		// Finalizer handling needs to come last.
		// We use a patch rather than an update to avoid conflicts when multiple controllers are removing their finalizer from the ClusterCR
		// We use the patch from sigs.k8s.io/cluster-api/util/patch to handle the patching without conflicts
		logger.Info("removing finalizer", "finalizer", monitoring.MonitoringFinalizer)
		patchHelper, err := patch.NewHelper(cluster, r.Client)
		if err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}

		controllerutil.RemoveFinalizer(cluster, monitoring.MonitoringFinalizer)
		if err := patchHelper.Patch(ctx, cluster); err != nil {
			logger.Error(err, "failed to remove finalizer, requeuing", "finalizer", monitoring.MonitoringFinalizer)
			return ctrl.Result{}, errors.WithStack(err)
		}
		logger.Info("removed finalizer", "finalizer", monitoring.MonitoringFinalizer)
	}
	return ctrl.Result{}, nil
}

// tearDown tears down the monitoring stack management cluster specific components like the hearbeat, mimir secrets and so on.
func (r *ClusterMonitoringReconciler) tearDown(ctx context.Context) error {
	logger := log.FromContext(ctx)

	err := r.HeartbeatRepository.Delete(ctx)
	if err != nil {
		logger.Error(err, "failed to delete heartbeat")
		return err
	}

	err = r.MimirService.DeleteMimirSecrets(ctx)
	if err != nil {
		logger.Error(err, "failed to delete mimir ingress secret")
		return err
	}

	return nil
}
