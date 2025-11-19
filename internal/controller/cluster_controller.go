package controller

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/giantswarm/observability-operator/pkg/alerting/heartbeat"
	"github.com/giantswarm/observability-operator/pkg/auth"
	"github.com/giantswarm/observability-operator/pkg/bundle"
	"github.com/giantswarm/observability-operator/pkg/common/organization"
	"github.com/giantswarm/observability-operator/pkg/config"
	"github.com/giantswarm/observability-operator/pkg/monitoring"
	"github.com/giantswarm/observability-operator/pkg/monitoring/alloy"
)

// ClusterMonitoringReconciler reconciles a Cluster object
type ClusterMonitoringReconciler struct {
	// Client is the controller client.
	Client client.Client
	Config config.Config
	// AlloyService is the service which manages Alloy monitoring agent configuration.
	AlloyService alloy.Service
	// HeartbeatRepositories is the list of repositories for managing heartbeats.
	HeartbeatRepositories []heartbeat.HeartbeatRepository
	// MimirAuthManager manages mimir authentication secrets.
	MimirAuthManager auth.AuthManager
	// BundleConfigurationService is the service for configuring the observability bundle.
	BundleConfigurationService *bundle.BundleConfigurationService
	// FinalizerHelper is the helper for managing finalizers.
	finalizerHelper FinalizerHelper
}

func SetupClusterMonitoringReconciler(mgr manager.Manager, cfg config.Config) error {
	managerClient := mgr.GetClient()

	// Create list of heartbeat repositories
	var heartbeatRepositories []heartbeat.HeartbeatRepository

	// Create Cronitor heartbeat repository if both keys are provided
	if cfg.Environment.CronitorHeartbeatManagementKey != "" && cfg.Environment.CronitorHeartbeatPingKey != "" {
		cronitorRepository, err := heartbeat.NewCronitorHeartbeatRepository(cfg, nil)
		if err != nil {
			return fmt.Errorf("unable to create cronitor heartbeat repository: %w", err)
		}
		heartbeatRepositories = append(heartbeatRepositories, cronitorRepository)
	}

	// Ensure at least one heartbeat repository is configured
	if len(heartbeatRepositories) == 0 {
		return fmt.Errorf("no heartbeat repositories configured: both CronitorHeartbeatManagementKey and CronitorHeartbeatPingKey must be set")
	}

	organizationRepository := organization.NewNamespaceRepository(managerClient)

	mimirAuthManager := auth.NewAuthManager(
		managerClient,
		auth.NewConfig(
			"mimir-basic-auth",
			"mimir",
			"mimir",
			"mimir-gateway-ingress-auth",
			"mimir-gateway-httproute-auth",
		),
	)

	alloyService := alloy.Service{
		Client:                 managerClient,
		OrganizationRepository: organizationRepository,
		Config:                 cfg,
		AuthManager:            mimirAuthManager,
	}

	r := &ClusterMonitoringReconciler{
		Client:                     managerClient,
		Config:                     cfg,
		HeartbeatRepositories:      heartbeatRepositories,
		AlloyService:               alloyService,
		MimirAuthManager:           mimirAuthManager,
		BundleConfigurationService: bundle.NewBundleConfigurationService(managerClient, cfg),
		finalizerHelper:            NewFinalizerHelper(managerClient, monitoring.MonitoringFinalizer),
	}

	return r.SetupWithManager(mgr)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterMonitoringReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(mgr).
		Named("cluster").
		For(&clusterv1.Cluster{}).
		// Reconcile all clusters when the grafana organizations have changed to update agents configs with the new list of tenants where metrics are sent to.
		Watches(&v1alpha1.GrafanaOrganization{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, object client.Object) []reconcile.Request {
				var logger = log.FromContext(ctx)
				var clusters clusterv1.ClusterList

				err := mgr.GetClient().List(ctx, &clusters)
				if err != nil {
					logger.Error(err, "failed to list cluster CRs")
					return []reconcile.Request{}
				}

				requests := make([]reconcile.Request, 0, len(clusters.Items))
				for _, cluster := range clusters.Items {
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      cluster.Name,
							Namespace: cluster.Namespace,
						},
					})
				}
				return requests
			})).
		Complete(r)
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	return nil
}

//+kubebuilder:rbac:groups=cluster.giantswarm.io,resources=clusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.giantswarm.io,resources=clusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cluster.giantswarm.io,resources=clusters/finalizers,verbs=update

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
		return ctrl.Result{}, fmt.Errorf("failed to get cluster: %w", err)
	}

	// Linting is disabled for the following line as otherwise it fails with the following error:
	// "should not use built-in type string as key for value"
	logger := log.FromContext(ctx).WithValues("installation", r.Config.Cluster.Name) // nolint
	ctx = log.IntoContext(ctx, logger)

	if !r.Config.Monitoring.Enabled {
		logger.Info("monitoring is disabled at the installation level.")
	}

	if !r.Config.Monitoring.IsMonitored(cluster) {
		logger.Info("monitoring is disabled for this cluster.")
	}

	// Handle deletion reconciliation loop.
	if !cluster.DeletionTimestamp.IsZero() {
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
	finalizerAdded, err := r.finalizerHelper.EnsureAdded(ctx, cluster)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure finalizer is added: %w", err)
	}
	if finalizerAdded {
		return ctrl.Result{}, nil
	}

	// Management cluster specific configuration
	if cluster.Name == r.Config.Cluster.Name {
		result := r.reconcileManagementCluster(ctx)
		if result != nil {
			return *result, nil
		}
	}

	// We always configure the bundle, even if monitoring is disabled for the cluster.
	err = r.BundleConfigurationService.Configure(ctx, cluster)
	if err != nil {
		logger.Error(err, "failed to configure the observability-bundle")
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	// Cluster specific configuration
	if r.Config.Monitoring.IsMonitored(cluster) {
		// Ensure cluster has a password in the centralized mimir auth secret
		err = r.MimirService.AddClusterPassword(ctx, cluster.Name)
		if err != nil {
			logger.Error(err, "failed to add cluster password to mimir auth secret")
			return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
		}

		observabilityBundleVersion, err := r.BundleConfigurationService.GetObservabilityBundleAppVersion(ctx, cluster)
		if err != nil {
			logger.Error(err, "failed to configure get observability-bundle version")
			return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
		}

		// Ensure cluster has a password in the centralized mimir auth secret
		err = r.MimirAuthManager.AddClusterPassword(ctx, cluster.Name)
		if err != nil {
			logger.Error(err, "failed to add cluster password to mimir auth secret")
			return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
		}

		// Create or update Alloy monitoring configuration.
		err = r.AlloyService.ReconcileCreate(ctx, cluster, observabilityBundleVersion)
		if err != nil {
			logger.Error(err, "failed to create or update alloy monitoring config")
			return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
		}
	} else {
		// Remove cluster password from mimir auth secret when monitoring disabled
		err = r.MimirAuthManager.RemoveClusterPassword(ctx, cluster.Name)
		if err != nil {
			logger.Error(err, "failed to remove cluster password from mimir auth secret")
			return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
		}

		// Clean up any existing alloy monitoring configuration
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
		if r.Config.Monitoring.IsMonitored(cluster) {
			// Delete Alloy monitoring configuration.
			err = r.AlloyService.ReconcileDelete(ctx, cluster)
			if err != nil {
				logger.Error(err, "failed to delete alloy monitoring config")
				return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
			}

			// Remove cluster password from mimir auth secret on cluster deletion
			err = r.MimirAuthManager.RemoveClusterPassword(ctx, cluster.Name)
			if err != nil {
				logger.Error(err, "failed to remove cluster password from mimir auth secret")
				return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
			}
		}
		// TODO add deletion of rules in the Mimir ruler on cluster deletion

		// Management cluster specific configuration
		if cluster.Name == r.Config.Cluster.Name {
			err := r.tearDown(ctx)
			if err != nil {
				logger.Error(err, "failed to tear down the monitoring stack")
				return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
			}
		}

		// We get the latest state of the object to avoid race conditions.
		// Finalizer handling needs to come last.
		err = r.finalizerHelper.EnsureRemoved(ctx, cluster)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

func (r *ClusterMonitoringReconciler) reconcileManagementCluster(ctx context.Context) *ctrl.Result {
	logger := log.FromContext(ctx)

	// If monitoring is enabled as the installation level, configure the monitoring stack, otherwise, tear it down.
	if r.Config.Monitoring.Enabled {
		for i, heartbeatRepo := range r.HeartbeatRepositories {
			err := heartbeatRepo.CreateOrUpdate(ctx)
			if err != nil {
				logger.Error(err, "failed to create or update heartbeat", "repository_index", i)
				return &ctrl.Result{RequeueAfter: 5 * time.Minute}
			}
		}
	} else {
		err := r.tearDown(ctx)
		if err != nil {
			logger.Error(err, "failed to tear down the monitoring stack")
			return &ctrl.Result{RequeueAfter: 5 * time.Minute}
		}
	}

	return nil
}

// tearDown tears down the monitoring stack management cluster specific components like the hearbeat, mimir secrets and so on.
func (r *ClusterMonitoringReconciler) tearDown(ctx context.Context) error {
	for i, heartbeatRepo := range r.HeartbeatRepositories {
		err := heartbeatRepo.Delete(ctx)
		if err != nil {
			return fmt.Errorf("failed to delete heartbeat (repository %d): %w", i, err)
		}
	}

	err := r.MimirAuthManager.DeleteAllSecrets(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete mimir secrets: %w", err)
	}

	return nil
}
