package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"

	"github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/giantswarm/observability-operator/pkg/agent"
	"github.com/giantswarm/observability-operator/pkg/agent/collectors"
	"github.com/giantswarm/observability-operator/pkg/agent/collectors/events"
	"github.com/giantswarm/observability-operator/pkg/agent/collectors/logs"
	"github.com/giantswarm/observability-operator/pkg/agent/collectors/metrics"
	agentcommon "github.com/giantswarm/observability-operator/pkg/agent/common"
	"github.com/giantswarm/observability-operator/pkg/alerting/heartbeat"
	"github.com/giantswarm/observability-operator/pkg/bundle"
	"github.com/giantswarm/observability-operator/pkg/common/organization"
	"github.com/giantswarm/observability-operator/pkg/common/tenancy"
	"github.com/giantswarm/observability-operator/pkg/config"
	"github.com/giantswarm/observability-operator/pkg/credential"
	operatormetrics "github.com/giantswarm/observability-operator/pkg/metrics"
	"github.com/giantswarm/observability-operator/pkg/monitoring"
	"github.com/giantswarm/observability-operator/pkg/ruler"
)

// collectorEntry pairs a CollectorService with the feature-flag predicate
// that determines whether that collector should be active for a given cluster.
type collectorEntry struct {
	name      string
	service   collectors.CollectorService
	isEnabled func(*clusterv1.Cluster) bool
}

// credentialEntry pairs an AgentCredential backend with the feature-flag
// predicate that determines whether a credential for that backend should
// exist for a given cluster.
type credentialEntry struct {
	backend   v1alpha1.CredentialBackend
	isEnabled func(*clusterv1.Cluster) bool
}

// ClusterReconciler reconciles a Cluster object
type ClusterReconciler struct {
	// Client is the controller client.
	Client client.Client
	Config config.Config
	// collectors is the ordered list of Alloy signal collectors (metrics, logs, events).
	collectors []collectorEntry
	// credentials is the ordered list of AgentCredential backends managed per cluster.
	credentials []credentialEntry
	// heartbeatRepositories is the list of repositories for managing heartbeats.
	heartbeatRepositories []heartbeat.HeartbeatRepository
	// observabilityBundleService is the service for configuring the observability bundle.
	observabilityBundleService bundle.ObservabilityBundleService
	// rulerClient deletes ruler rules on cluster deletion.
	rulerClient ruler.Client
	// tenantRepository provides the list of all active tenants for ruler cleanup.
	tenantRepository tenancy.TenantRepository
	// finalizerHelper is the helper for managing finalizers.
	finalizerHelper FinalizerHelper
}

func SetupClusterReconciler(mgr manager.Manager, cfg config.Config, logger logr.Logger) error {
	managerClient := mgr.GetClient()

	// Create list of heartbeat repositories
	var heartbeatRepositories []heartbeat.HeartbeatRepository

	// Create Cronitor heartbeat repository if both keys are provided
	if cfg.Environment.CronitorHeartbeatManagementKey != "" && cfg.Environment.CronitorHeartbeatPingKey != "" {
		cronitorRepository := heartbeat.NewCronitorHeartbeatRepository(cfg, nil)
		heartbeatRepositories = append(heartbeatRepositories, cronitorRepository)
	}

	if len(heartbeatRepositories) == 0 {
		logger.Info("no heartbeat repositories configured (CronitorHeartbeatManagementKey/CronitorHeartbeatPingKey), disabling this feature")
	}

	organizationRepository := organization.NewNamespaceRepository(managerClient)
	tenantRepository := tenancy.NewTenantRepository(managerClient)

	// Create agent configuration repository
	agentConfigurationRepository := agent.NewConfigurationRepository(managerClient)
	credentialReader := credential.NewReader(managerClient)

	alloyMetricsService := &metrics.Service{
		Config:                  cfg,
		ConfigurationRepository: agentConfigurationRepository,
		OrganizationRepository:  organizationRepository,
		TenantRepository:        tenantRepository,
		CredentialReader:        credentialReader,
	}

	alloyLogsService := &logs.Service{
		Config:                  cfg,
		ConfigurationRepository: agentConfigurationRepository,
		OrganizationRepository:  organizationRepository,
		TenantRepository:        tenantRepository,
		CredentialReader:        credentialReader,
	}

	alloyEventsService := &events.Service{
		Config:                  cfg,
		ConfigurationRepository: agentConfigurationRepository,
		OrganizationRepository:  organizationRepository,
		TenantRepository:        tenantRepository,
		CredentialReader:        credentialReader,
	}

	agentCredentials := []credentialEntry{
		{backend: v1alpha1.CredentialBackendMetrics, isEnabled: cfg.Monitoring.IsMonitoringEnabled},
		{backend: v1alpha1.CredentialBackendLogs, isEnabled: cfg.Logging.IsLoggingEnabled},
		{backend: v1alpha1.CredentialBackendTraces, isEnabled: cfg.Tracing.IsTracingEnabled},
	}

	alloyCollectors := []collectorEntry{
		{
			name:      "metrics",
			service:   alloyMetricsService,
			isEnabled: cfg.Monitoring.IsMonitoringEnabled,
		},
		{
			// alloy-logs handles logs and network monitoring (daemonset, one per node)
			name:    "logs",
			service: alloyLogsService,
			isEnabled: func(c *clusterv1.Cluster) bool {
				return cfg.Logging.IsLoggingEnabled(c) || cfg.Monitoring.IsNetworkMonitoringEnabled(c)
			},
		},
		{
			// alloy-events handles kube events, traces and OTLP (deployment, cluster-level)
			name:    "events",
			service: alloyEventsService,
			isEnabled: func(c *clusterv1.Cluster) bool {
				return cfg.Logging.IsLoggingEnabled(c) || cfg.Tracing.IsTracingEnabled(c) || cfg.Monitoring.IsMonitoringEnabled(c)
			},
		},
	}

	var rulerClients []ruler.Client
	if cfg.Monitoring.RulerURL != "" {
		rulerClients = append(rulerClients, ruler.NewMimir(cfg.Monitoring.RulerURL, cfg.HTTP.RulerTimeout))
	}
	if cfg.Logging.RulerURL != "" {
		rulerClients = append(rulerClients, ruler.NewLoki(cfg.Logging.RulerURL, cfg.HTTP.RulerTimeout))
	}
	rulerClient := ruler.NewMulti(rulerClients...)

	r := &ClusterReconciler{
		Client:                     managerClient,
		Config:                     cfg,
		collectors:                 alloyCollectors,
		credentials:                agentCredentials,
		heartbeatRepositories:      heartbeatRepositories,
		observabilityBundleService: bundle.New(managerClient, cfg),
		rulerClient:                rulerClient,
		tenantRepository:           tenantRepository,
		finalizerHelper:            NewFinalizerHelper(managerClient, monitoring.MonitoringFinalizer),
	}

	return r.SetupWithManager(mgr)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(mgr).
		Named("cluster").
		For(&clusterv1.Cluster{}).
		// Reconcile all clusters when the CA Secret changes so Alloy picks up CA rotation immediately.
		Watches(&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, object client.Object) []reconcile.Request {
				if r.Config.Cluster.CASecretName == "" ||
					object.GetNamespace() != r.Config.Cluster.CASecretNamespace ||
					object.GetName() != r.Config.Cluster.CASecretName {
					return nil
				}
				logger := log.FromContext(ctx)
				var clusters clusterv1.ClusterList
				if err := mgr.GetClient().List(ctx, &clusters); err != nil {
					logger.Error(err, "failed to list cluster CRs on CA Secret change")
					return nil
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
		// Reconcile all clusters when the grafana organizations have changed to update agents configs with the new list of tenants where metrics are sent to.
		Watches(&v1alpha1.GrafanaOrganization{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, object client.Object) []reconcile.Request {
				logger := log.FromContext(ctx)
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
		WithOptions(controller.Options{
			RateLimiter: workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](
				1*time.Second, // base delay
				5*time.Minute, // max delay
			),
		}).
		Complete(r)
	if err != nil {
		return fmt.Errorf("failed to build controller: %w", err)
	}

	return nil
}

//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch
//+kubebuilder:rbac:groups=application.giantswarm.io,resources=apps,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=helm.toolkit.fluxcd.io,resources=helmreleases,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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
		logger.Info("monitoring is disabled at the installation level")
	}

	if !r.Config.Monitoring.IsMonitoringEnabled(cluster) {
		logger.Info("monitoring is disabled for this cluster", "cluster", cluster.Name, "namespace", cluster.Namespace)
	}

	// Handle deletion reconciliation loop.
	if !cluster.DeletionTimestamp.IsZero() {
		logger.Info("handling deletion for cluster", "cluster", cluster.Name, "namespace", cluster.Namespace)
		return r.reconcileDelete(ctx, cluster)
	}

	logger.Info("reconciling cluster", "cluster", cluster.Name, "namespace", cluster.Namespace)

	// Handle normal reconciliation loop.
	return r.reconcile(ctx, cluster)
}

// reconcile handles cluster reconciliation.
func (r *ClusterReconciler) reconcile(ctx context.Context, cluster *clusterv1.Cluster) (ctrl.Result, error) {
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

	// Collect all errors to ensure all independent tasks have a chance to run
	var errs []error

	// We always configure the bundle, even if monitoring is disabled for the cluster.
	err = r.observabilityBundleService.Configure(ctx, cluster)
	if err != nil {
		logger.Error(err, "failed to configure the observability-bundle")
		errs = append(errs, fmt.Errorf("bundle configuration: %w", err))
	}

	// Reconcile alloy services (bundled dependent tasks)
	err = r.reconcileAlloyServices(ctx, cluster)
	if err != nil {
		logger.Error(err, "failed to reconcile alloy services")
		errs = append(errs, fmt.Errorf("alloy services: %w", err))
	}

	// Management cluster specific configuration
	if cluster.Name == r.Config.Cluster.Name {
		_, err := r.reconcileManagementCluster(ctx)
		if err != nil {
			errs = append(errs, fmt.Errorf("management cluster reconciliation: %w", err))
		}
	}

	// If any errors occurred, combine them and return
	if len(errs) > 0 {
		return ctrl.Result{}, errors.Join(errs...)
	}

	operatormetrics.MonitoredClusterInfo.WithLabelValues(cluster.Name, cluster.Namespace).Set(1)

	return ctrl.Result{}, nil
}

// reconcileAlloyServices reconciles all alloy services. This is a bundled operation
// where getting the observability bundle version is a dependency for configuring
// all alloy services. If getting the version fails, all dependent tasks fail.
func (r *ClusterReconciler) reconcileAlloyServices(ctx context.Context, cluster *clusterv1.Cluster) error {
	logger := log.FromContext(ctx)

	// Reconcile per-backend AgentCredential CRs. Each CR is owned by the Cluster;
	// the AgentCredentialReconciler mints the Secret and keeps the gateway
	// htpasswd in sync.
	if err := r.reconcileAgentCredentials(ctx, cluster); err != nil {
		return err
	}

	// Get bundle version - this is required for all alloy service operations
	observabilityBundleVersion, err := r.observabilityBundleService.GetBundleVersion(ctx, cluster)
	if err != nil {
		return fmt.Errorf("failed to get observability-bundle version: %w", err)
	}

	// Read CA bundle once for all services (empty string on public-CA installations).
	caBundle, err := agentcommon.ReadCABundle(ctx, r.Client, r.Config.Cluster)
	if err != nil {
		return fmt.Errorf("failed to read CA bundle: %w", err)
	}

	// Reconcile each collector: create if enabled, delete if disabled (independent tasks).
	var errs []error
	for _, c := range r.collectors {
		if c.isEnabled(cluster) {
			if err := c.service.ReconcileCreate(ctx, cluster, observabilityBundleVersion, caBundle); err != nil {
				logger.Error(err, "failed to create or update alloy config", "collector", c.name)
				errs = append(errs, fmt.Errorf("alloy %s reconcile create: %w", c.name, err))
			}
		} else {
			if err := c.service.ReconcileDelete(ctx, cluster); err != nil {
				logger.Error(err, "failed to delete alloy config", "collector", c.name)
				errs = append(errs, fmt.Errorf("alloy %s reconcile delete: %w", c.name, err))
			}
		}
	}

	return errors.Join(errs...)
}

// reconcileAgentCredentials ensures an AgentCredential CR exists per enabled
// backend and is removed for disabled backends.
func (r *ClusterReconciler) reconcileAgentCredentials(ctx context.Context, cluster *clusterv1.Cluster) error {
	var errs []error
	for _, c := range r.credentials {
		if c.isEnabled(cluster) {
			if err := r.ensureAgentCredential(ctx, cluster, c.backend); err != nil {
				errs = append(errs, fmt.Errorf("ensure agent credential for %s: %w", c.backend, err))
			}
		} else {
			if err := r.deleteAgentCredential(ctx, cluster, c.backend); err != nil {
				errs = append(errs, fmt.Errorf("delete agent credential for %s: %w", c.backend, err))
			}
		}
	}
	return errors.Join(errs...)
}

func (r *ClusterReconciler) ensureAgentCredential(ctx context.Context, cluster *clusterv1.Cluster, backend v1alpha1.CredentialBackend) error {
	cred := &v1alpha1.AgentCredential{
		ObjectMeta: metav1.ObjectMeta{
			Name:      credential.ClusterCredentialName(cluster.Name, backend),
			Namespace: cluster.Namespace,
		},
	}

	_, err := ctrl.CreateOrUpdate(ctx, r.Client, cred, func() error {
		if err := controllerutil.SetControllerReference(cluster, cred, r.Client.Scheme()); err != nil {
			return fmt.Errorf("failed to set owner reference: %w", err)
		}
		cred.Spec.Backend = backend
		cred.Spec.AgentName = cluster.Name
		cred.Spec.SecretName = credential.ClusterSecretName(cluster.Name, backend)
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to upsert agent credential: %w", err)
	}
	return nil
}

func (r *ClusterReconciler) deleteAgentCredential(ctx context.Context, cluster *clusterv1.Cluster, backend v1alpha1.CredentialBackend) error {
	key := client.ObjectKey{
		Name:      credential.ClusterCredentialName(cluster.Name, backend),
		Namespace: cluster.Namespace,
	}
	cred := &v1alpha1.AgentCredential{}
	// Get-then-Delete avoids a wasted API write per reconcile for backends that
	// were never enabled on this cluster (which is the common steady state).
	if err := r.Client.Get(ctx, key, cred); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get agent credential: %w", err)
	}
	if err := client.IgnoreNotFound(r.Client.Delete(ctx, cred)); err != nil {
		return fmt.Errorf("failed to delete agent credential: %w", err)
	}
	return nil
}

// reconcileDelete handles cluster deletion.
func (r *ClusterReconciler) reconcileDelete(ctx context.Context, cluster *clusterv1.Cluster) (reconcile.Result, error) {
	logger := log.FromContext(ctx)

	// We do not need to delete anything if there is no finalizer on the cluster
	if !controllerutil.ContainsFinalizer(cluster, monitoring.MonitoringFinalizer) {
		return ctrl.Result{}, nil
	}

	// Collect all errors to ensure all cleanup tasks have a chance to run
	var errs []error

	// We always remove the bundle configure, even if monitoring is disabled for the cluster.
	err := r.observabilityBundleService.RemoveConfiguration(ctx, cluster)
	if err != nil {
		logger.Error(err, "failed to remove the observability-bundle configuration")
		errs = append(errs, fmt.Errorf("remove bundle configuration: %w", err))
	}

	// Delete Alloy configuration for every collector unconditionally. A feature
	// flag flipped off between the previous reconcile and the delete would
	// otherwise leak that collector's ConfigMap/Secret. Each ReconcileDelete
	// uses client.IgnoreNotFound internally, so deleting "nothing" is a no-op.
	for _, c := range r.collectors {
		if err := c.service.ReconcileDelete(ctx, cluster); err != nil {
			logger.Error(err, "failed to delete alloy config", "collector", c.name)
			errs = append(errs, fmt.Errorf("delete alloy %s: %w", c.name, err))
		}
	}

	// Explicitly request deletion of the per-backend AgentCredential CRs.
	//
	// Ordering note: this returns as soon as the API server has acknowledged
	// the delete request, but each AgentCredential carries its own finalizer
	// and stays around until the AgentCredentialReconciler re-aggregates the
	// gateway htpasswd. The Cluster's finalizer is removed below in the same
	// reconcile pass, so the Cluster object can disappear while its child ACs
	// are still terminating — that is intentional and safe: the AC reconciler
	// owns gateway-secret cleanup independently.
	for _, c := range r.credentials {
		if err := r.deleteAgentCredential(ctx, cluster, c.backend); err != nil {
			logger.Error(err, "failed to delete agent credential", "backend", c.backend)
			errs = append(errs, err)
		}
	}

	// Delete ruler rules scoped to this cluster for every active tenant.
	tenants, err := r.tenantRepository.List(ctx)
	if err != nil {
		logger.Error(err, "failed to list tenants for ruler cleanup")
		errs = append(errs, fmt.Errorf("list tenants for ruler cleanup: %w", err))
	} else {
		for _, tenantID := range tenants {
			if err := r.rulerClient.DeleteClusterRulesForTenant(ctx, tenantID, cluster.Name); err != nil {
				logger.Error(err, "failed to delete ruler rules", "tenant", tenantID)
				errs = append(errs, fmt.Errorf("delete ruler rules for tenant %s: %w", tenantID, err))
			}
		}
	}

	// Management cluster specific configuration
	if cluster.Name == r.Config.Cluster.Name {
		err := r.tearDown(ctx)
		if err != nil {
			logger.Error(err, "failed to tear down the monitoring stack")
			errs = append(errs, fmt.Errorf("teardown monitoring stack: %w", err))
		}
	}

	// If any errors occurred during deletion, combine them and return
	if len(errs) > 0 {
		return ctrl.Result{}, errors.Join(errs...)
	}

	// We get the latest state of the object to avoid race conditions.
	// Finalizer handling needs to come last.
	err = r.finalizerHelper.EnsureRemoved(ctx, cluster)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
	}

	operatormetrics.MonitoredClusterInfo.DeleteLabelValues(cluster.Name, cluster.Namespace)

	return ctrl.Result{}, nil
}

func (r *ClusterReconciler) reconcileManagementCluster(ctx context.Context) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Collect all errors to ensure all tasks have a chance to run
	var errs []error

	// If monitoring is enabled as the installation level, configure the monitoring stack, otherwise, tear it down.
	if r.Config.Monitoring.Enabled {
		if len(r.heartbeatRepositories) == 0 {
			logger.Info("no heartbeat repositories configured, skipping this feature")
		}

		for i, heartbeatRepo := range r.heartbeatRepositories {
			err := heartbeatRepo.CreateOrUpdate(ctx)
			if err != nil {
				logger.Error(err, "failed to create or update heartbeat", "repository_index", i)
				errs = append(errs, fmt.Errorf("heartbeat repository %d: %w", i, err))
			}
		}
	} else {
		err := r.tearDown(ctx)
		if err != nil {
			logger.Error(err, "failed to tear down the monitoring stack")
			errs = append(errs, fmt.Errorf("teardown monitoring stack: %w", err))
		}
	}

	// If any errors occurred, combine them and return
	if len(errs) > 0 {
		return ctrl.Result{}, errors.Join(errs...)
	}

	return ctrl.Result{}, nil
}

// tearDown tears down management-cluster-specific components (heartbeat).
// Gateway htpasswd entries are cleaned up implicitly by the AgentCredential
// reconciler as each per-cluster credential is deleted: the aggregated gateway
// Secret is rewritten with the remaining entries (empty once the last CR is gone).
func (r *ClusterReconciler) tearDown(ctx context.Context) error {
	for i, heartbeatRepo := range r.heartbeatRepositories {
		err := heartbeatRepo.Delete(ctx)
		if err != nil {
			return fmt.Errorf("failed to delete heartbeat (repository %d): %w", i, err)
		}
	}
	return nil
}
