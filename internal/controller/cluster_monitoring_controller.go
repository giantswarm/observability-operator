package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/blang/semver"
	appv1alpha1 "github.com/giantswarm/apiextensions-application/api/v1alpha1"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/giantswarm/observability-operator/internal/controller/predicates"
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
	"github.com/giantswarm/observability-operator/pkg/rules"
)

var (
	observabilityBundleVersionSupportAlloyMetrics = semver.MustParse("1.6.2")
)

// ClusterMonitoringReconciler reconciles a Cluster object
type ClusterMonitoringReconciler struct {
	// Client is the controller client.
	client.Client
	common.ManagementCluster
	// PrometheusAgentService is the service for managing PrometheusAgent resources.
	prometheusagent.PrometheusAgentService
	// AlloyRulesService is the service used to configure the alloy-rules instance.
	AlloyRulesService rules.Service
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
		PasswordManager:        password.SimpleManager{},
		ManagementCluster:      conf.ManagementCluster,
		MonitoringConfig:       conf.Monitoring,
	}

	mimirService := mimir.MimirService{
		Client:            managerClient,
		PasswordManager:   password.SimpleManager{},
		ManagementCluster: conf.ManagementCluster,
	}

	alloyRulesService := rules.Service{
		Client: managerClient,
	}

	r := &ClusterMonitoringReconciler{
		Client:                     managerClient,
		ManagementCluster:          conf.ManagementCluster,
		HeartbeatRepository:        heartbeatRepository,
		PrometheusAgentService:     prometheusAgentService,
		AlloyRulesService:          alloyRulesService,
		AlloyService:               alloyService,
		MimirService:               mimirService,
		MonitoringConfig:           conf.Monitoring,
		BundleConfigurationService: bundle.NewBundleConfigurationService(managerClient, conf.Monitoring),
	}

	err = r.SetupWithManager(mgr, conf.ManagementCluster.Name)
	if err != nil {
		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterMonitoringReconciler) SetupWithManager(mgr ctrl.Manager, managementClusterName string) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1.Cluster{}).
		// This ensures we run the reconcile loop when the alloy-rules app is redeployed from prometheus-rules.
		Watches(
			&appv1alpha1.App{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, object client.Object) []reconcile.Request {
				return []reconcile.Request{
					{NamespacedName: types.NamespacedName{
						Name:      managementClusterName,
						Namespace: "org-giantswarm",
					}},
				}
			}),
			builder.WithPredicates(predicates.AlloyRulesAppChangedPredicate{}),
		).
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
		return r.addFinalizer(ctx, cluster)
	}

	// Management cluster specific configuration
	if cluster.Name == r.ManagementCluster.Name {
		result := r.reconcileManagementCluster(ctx, cluster)
		if result != nil {
			return *result, nil
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
			return ctrl.Result{}, errors.Errorf("unsupported monitoring agent %q", monitoringAgent)
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
			err := r.tearDown(ctx, cluster)
			if err != nil {
				logger.Error(err, "failed to tear down the monitoring stack")
				return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
			}
		}

		// We get the latest state of the object to avoid race conditions.
		// Finalizer handling needs to come last.
		err = r.removeFinalizer(ctx, cluster)
		if err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
	}

	return ctrl.Result{}, nil
}

func (r *ClusterMonitoringReconciler) addFinalizer(ctx context.Context, cluster *clusterv1.Cluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

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

func (r *ClusterMonitoringReconciler) removeFinalizer(ctx context.Context, cluster *clusterv1.Cluster) error {
	logger := log.FromContext(ctx)

	// We use a patch rather than an update to avoid conflicts when multiple controllers are removing their finalizer from the ClusterCR
	// We use the patch from sigs.k8s.io/cluster-api/util/patch to handle the patching without conflicts
	logger.Info("removing finalizer", "finalizer", monitoring.MonitoringFinalizer)
	patchHelper, err := patch.NewHelper(cluster, r.Client)
	if err != nil {
		return errors.WithStack(err)
	}

	controllerutil.RemoveFinalizer(cluster, monitoring.MonitoringFinalizer)
	if err := patchHelper.Patch(ctx, cluster); err != nil {
		logger.Error(err, "failed to remove finalizer, requeuing", "finalizer", monitoring.MonitoringFinalizer)
		return errors.WithStack(err)
	}
	logger.Info("removed finalizer", "finalizer", monitoring.MonitoringFinalizer)
	return nil
}

func (r *ClusterMonitoringReconciler) reconcileManagementCluster(ctx context.Context, cluster *clusterv1.Cluster) *ctrl.Result {
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

		err = r.AlloyRulesService.Configure(ctx, *cluster)
		if err != nil {
			logger.Error(err, "failed to configure alloy-rules")
			return &ctrl.Result{RequeueAfter: 5 * time.Minute}
		}
	} else {
		err := r.tearDown(ctx, cluster)
		if err != nil {
			logger.Error(err, "failed to tear down the monitoring stack")
			return &ctrl.Result{RequeueAfter: 5 * time.Minute}
		}
	}

	return nil
}

// tearDown tears down the monitoring stack management cluster specific components like the hearbeat, mimir secrets and so on.
func (r *ClusterMonitoringReconciler) tearDown(ctx context.Context, cluster *clusterv1.Cluster) error {
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

	err = r.AlloyRulesService.CleanUp(ctx, *cluster)
	if err != nil {
		logger.Error(err, "failed to clean up alloy-rules config")
		return err
	}

	return nil
}
