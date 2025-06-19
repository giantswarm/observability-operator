package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/blang/semver/v4"
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
	"github.com/giantswarm/observability-operator/pkg/bundle"
	"github.com/giantswarm/observability-operator/pkg/common"
	commonmonitoring "github.com/giantswarm/observability-operator/pkg/common/monitoring"
	"github.com/giantswarm/observability-operator/pkg/common/organization"
	"github.com/giantswarm/observability-operator/pkg/common/password"
	"github.com/giantswarm/observability-operator/pkg/config"
	"github.com/giantswarm/observability-operator/pkg/monitoring"
	"github.com/giantswarm/observability-operator/pkg/monitoring/alloy"
	"github.com/giantswarm/observability-operator/pkg/monitoring/heartbeat"
	"github.com/giantswarm/observability-operator/pkg/monitoring/mimir"
	"github.com/giantswarm/observability-operator/pkg/monitoring/prometheusagent"
)

var (
	observabilityBundleVersionSupportAlloyMetrics = semver.MustParse("1.6.2")
)

// ClusterMonitoringReconciler reconciles a Cluster object
type ClusterMonitoringReconciler struct {
	// Client is the controller client.
	Client            client.Client
	ManagementCluster common.ManagementCluster
	// PrometheusAgentService is the service for managing PrometheusAgent resources.
	PrometheusAgentService prometheusagent.PrometheusAgentService
	// AlloyService is the service which manages Alloy monitoring agent configuration.
	AlloyService alloy.Service
	// HeartbeatRepository is the repository for managing heartbeats.
	HeartbeatRepository heartbeat.HeartbeatRepository
	// MimirService is the service for managing mimir configuration.
	MimirService mimir.MimirService
	// BundleConfigurationService is the service for configuring the observability bundle.
	BundleConfigurationService *bundle.BundleConfigurationService
	// MonitoringConfig is the configuration for the monitoring package.
	MonitoringConfig monitoring.Config
	// FinalizerHelper is the helper for managing finalizers.
	finalizerHelper FinalizerHelper
}

func SetupClusterMonitoringReconciler(mgr manager.Manager, conf config.Config) error {
	managerClient := mgr.GetClient()

	if conf.Environment.OpsgenieApiKey == "" {
		return fmt.Errorf("OpsgenieApiKey not set: %q", conf.Environment.OpsgenieApiKey)
	}

	heartbeatRepository, err := heartbeat.NewOpsgenieHeartbeatRepository(conf.Environment.OpsgenieApiKey, conf.ManagementCluster)
	if err != nil {
		return fmt.Errorf("unable to create heartbeat repository: %w", err)
	}

	organizationRepository := organization.NewNamespaceRepository(managerClient)

	prometheusAgentService := prometheusagent.PrometheusAgentService{
		Client:                 managerClient,
		OrganizationRepository: organizationRepository,
		PasswordManager:        password.SimpleManager{},
		ManagementCluster:      conf.ManagementCluster,
		MonitoringConfig:       conf.Monitoring,
	}

	alloyService := alloy.Service{
		Client:                 managerClient,
		OrganizationRepository: organizationRepository,
		ManagementCluster:      conf.ManagementCluster,
		MonitoringConfig:       conf.Monitoring,
	}

	mimirService := mimir.MimirService{
		Client:            managerClient,
		PasswordManager:   password.SimpleManager{},
		ManagementCluster: conf.ManagementCluster,
	}

	r := &ClusterMonitoringReconciler{
		Client:                     managerClient,
		ManagementCluster:          conf.ManagementCluster,
		HeartbeatRepository:        heartbeatRepository,
		PrometheusAgentService:     prometheusAgentService,
		AlloyService:               alloyService,
		MimirService:               mimirService,
		MonitoringConfig:           conf.Monitoring,
		BundleConfigurationService: bundle.NewBundleConfigurationService(managerClient, conf.Monitoring),
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
	logger := log.FromContext(ctx).WithValues("installation", r.ManagementCluster.Name) // nolint
	ctx = log.IntoContext(ctx, logger)

	if !r.MonitoringConfig.Enabled {
		logger.Info("monitoring is disabled at the installation level.")
	}

	if !r.MonitoringConfig.IsMonitored(cluster) {
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
	if err != nil || finalizerAdded {
		return ctrl.Result{}, fmt.Errorf("failed to ensure finalizer is added: %w", err)
	}

	// Management cluster specific configuration
	if cluster.Name == r.ManagementCluster.Name {
		result := r.reconcileManagementCluster(ctx)
		if result != nil {
			return *result, nil
		}
	}

	// Enforce prometheus-agent as monitoring agent when observability-bundle version < 1.6.0
	monitoringAgent := r.MonitoringConfig.MonitoringAgent
	observabilityBundleVersion, err := r.BundleConfigurationService.GetObservabilityBundleAppVersion(ctx, cluster)
	if err != nil {
		logger.Error(err, "failed to configure get observability-bundle version")
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
	}
	if observabilityBundleVersion.LT(observabilityBundleVersionSupportAlloyMetrics) && monitoringAgent != commonmonitoring.MonitoringAgentPrometheus {
		logger.Info("Monitoring agent is not supported by observability bundle, using prometheus-agent instead.", "observability-bundle-version", observabilityBundleVersion, "monitoring-agent", monitoringAgent)
		monitoringAgent = commonmonitoring.MonitoringAgentPrometheus
	}

	// We always configure the bundle, even if monitoring is disabled for the cluster.
	err = r.BundleConfigurationService.Configure(ctx, cluster, monitoringAgent)
	if err != nil {
		logger.Error(err, "failed to configure the observability-bundle")
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	// Cluster specific configuration
	if r.MonitoringConfig.IsMonitored(cluster) {
		switch monitoringAgent {
		case commonmonitoring.MonitoringAgentPrometheus:
			// Create or update PrometheusAgent remote write configuration.
			err = r.PrometheusAgentService.ReconcileRemoteWriteConfiguration(ctx, cluster)
			if err != nil {
				logger.Error(err, "failed to create or update prometheus agent remote write config")
				return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
			}
		case commonmonitoring.MonitoringAgentAlloy:
			// Create or update Alloy monitoring configuration.
			err = r.AlloyService.ReconcileCreate(ctx, cluster, observabilityBundleVersion)
			if err != nil {
				logger.Error(err, "failed to create or update alloy monitoring config")
				return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
			}
		default:
			return ctrl.Result{}, fmt.Errorf("unsupported monitoring agent %q", monitoringAgent)
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
			// Delete PrometheusAgent remote write configuration.
			err := r.PrometheusAgentService.DeleteRemoteWriteConfiguration(ctx, cluster)
			if err != nil {
				logger.Error(err, "failed to delete prometheus agent remote write config")
				return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
			}
			// Delete Alloy monitoring configuration.
			err = r.AlloyService.ReconcileDelete(ctx, cluster)
			if err != nil {
				logger.Error(err, "failed to delete alloy monitoring config")
				return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
			}
		}

		// TODO add deletion of rules in the Mimir ruler on cluster deletion

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
	if r.MonitoringConfig.Enabled {
		err := r.HeartbeatRepository.CreateOrUpdate(ctx)
		if err != nil {
			logger.Error(err, "failed to create or update heartbeat")
			return &ctrl.Result{RequeueAfter: 5 * time.Minute}
		}

		err = r.MimirService.ConfigureMimir(ctx)
		if err != nil {
			logger.Error(err, "failed to configure mimir")
			return &ctrl.Result{RequeueAfter: 5 * time.Minute}
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
	err := r.HeartbeatRepository.Delete(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete heartbeat: %w", err)
	}

	err = r.MimirService.DeleteMimirSecrets(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete mimir ingress secret: %w", err)
	}

	return nil
}
