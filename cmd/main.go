package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net/url"
	"os"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/Netflix/go-env"
	appv1 "github.com/giantswarm/apiextensions-application/api/v1alpha1"
	pflag "github.com/spf13/pflag"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
	observabilityv1alpha2 "github.com/giantswarm/observability-operator/api/v1alpha2"
	"github.com/giantswarm/observability-operator/internal/controller"
	webhookcorev1 "github.com/giantswarm/observability-operator/internal/webhook/v1"
	webhookcorev1alpha1 "github.com/giantswarm/observability-operator/internal/webhook/v1alpha1"
	webhookcorev1alpha2 "github.com/giantswarm/observability-operator/internal/webhook/v1alpha2"
	"github.com/giantswarm/observability-operator/pkg/config"
	grafanaclient "github.com/giantswarm/observability-operator/pkg/grafana/client"
	//+kubebuilder:scaffold:imports
)

const (
	// Operator configuration flag names
	flagMetricsBindAddress     = "metrics-bind-address"
	flagHealthProbeBindAddress = "health-probe-bind-address"
	flagLeaderElect            = "leader-elect"
	flagMetricsSecure          = "metrics-secure"
	flagEnableHTTP2            = "enable-http2"
	flagWebhookCertPath        = "webhook-cert-path"
	flagOperatorNamespace      = "operator-namespace"

	// Grafana configuration flag names
	flagGrafanaURL = "grafana-url"

	// Management cluster configuration flag names
	flagManagementClusterBaseDomain = "management-cluster-base-domain"
	flagManagementClusterCustomer   = "management-cluster-customer"
	flagManagementClusterInsecureCA = "management-cluster-insecure-ca"
	flagManagementClusterName       = "management-cluster-name"
	flagManagementClusterPipeline   = "management-cluster-pipeline"
	flagManagementClusterRegion     = "management-cluster-region"

	// Monitoring configuration flag names
	flagAlertmanagerEnabled                   = "alertmanager-enabled"
	flagAlertmanagerSecretName                = "alertmanager-secret-name"
	flagAlertmanagerURL                       = "alertmanager-url"
	flagMonitoringEnabled                     = "monitoring-enabled"
	flagMonitoringShardingScaleUpSeriesCount  = "monitoring-sharding-scale-up-series-count"
	flagMonitoringShardingScaleDownPercentage = "monitoring-sharding-scale-down-percentage"
	flagMonitoringWALTruncateFrequency        = "monitoring-wal-truncate-frequency"
	flagMonitoringMetricsQueryURL             = "monitoring-metrics-query-url"
	// TODO Rename the flag with the monitoring prefix when migration is done
	flagMonitoringNetworkEnabled = "logging-enable-network-monitoring"

	// Queue configuration flag names
	flagQueueBatchSendDeadline = "monitoring-queue-config-batch-send-deadline"
	flagQueueCapacity          = "monitoring-queue-config-capacity"
	flagQueueMaxBackoff        = "monitoring-queue-config-max-backoff"
	flagQueueMaxSamplesPerSend = "monitoring-queue-config-max-samples-per-send"
	flagQueueMaxShards         = "monitoring-queue-config-max-shards"
	flagQueueMinBackoff        = "monitoring-queue-config-min-backoff"
	flagQueueMinShards         = "monitoring-queue-config-min-shards"
	flagQueueRetryOnHttp429    = "monitoring-queue-config-retry-on-http-429"
	flagQueueSampleAgeLimit    = "monitoring-queue-config-sample-age-limit"

	// Tracing configuration flag names
	flagTracingEnabled = "tracing-enabled"

	// Logging configuration flag names
	flagLoggingEnabled                     = "logging-enabled"
	flagLoggingEnableEvents                = "logging-enable-alloy-events-reconciliation"
	flagLoggingDefaultNamespaces           = "logging-default-namespaces"
	flagLoggingEnableNodeFiltering         = "logging-enable-node-filtering"
	flagLoggingIncludeEventsFromNamespaces = "logging-include-events-from-namespaces"
	flagLoggingExcludeEventsFromNamespaces = "logging-exclude-events-from-namespaces"
)

var (
	cfg config.Config

	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(appv1.AddToScheme(scheme))
	utilruntime.Must(observabilityv1alpha1.AddToScheme(scheme))
	utilruntime.Must(observabilityv1alpha2.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	err := runner()
	if err != nil {
		setupLog.Error(err, "observability-operator failed")
		os.Exit(1)
	}
}

func runner() error {
	// Parse flags
	if err := parseFlags(); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Continue with the rest of the setup
	return setupApplication()
}

// parseFlags parses all command line flags and updates the configuration.
func parseFlags() (err error) {
	// Operator configuration flags
	pflag.StringVar(&cfg.Operator.MetricsAddr, flagMetricsBindAddress, ":8080",
		"The address the metric endpoint binds to.")
	pflag.StringVar(&cfg.Operator.ProbeAddr, flagHealthProbeBindAddress, ":8081",
		"The address the probe endpoint binds to.")
	pflag.BoolVar(&cfg.Operator.EnableLeaderElection, flagLeaderElect, false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	pflag.BoolVar(&cfg.Operator.SecureMetrics, flagMetricsSecure, false,
		"If set the metrics endpoint is served securely")
	pflag.BoolVar(&cfg.Operator.EnableHTTP2, flagEnableHTTP2, false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	pflag.StringVar(&cfg.Operator.WebhookCertPath, flagWebhookCertPath, "/tmp/k8s-webhook-server/serving-certs",
		"Path to the directory where the webhook server will store its TLS certificate and key.")
	pflag.StringVar(&cfg.Operator.OperatorNamespace, flagOperatorNamespace, "",
		"The namespace where the observability-operator is running.")

	// Grafana configuration flags
	var grafanaURL string
	pflag.StringVar(&grafanaURL, flagGrafanaURL, "http://grafana.monitoring.svc.cluster.local",
		"grafana URL")

	// Management cluster configuration flags
	pflag.StringVar(&cfg.Cluster.BaseDomain, flagManagementClusterBaseDomain, "",
		"The base domain of the management cluster.")
	pflag.StringVar(&cfg.Cluster.Customer, flagManagementClusterCustomer, "",
		"The customer of the management cluster.")
	pflag.BoolVar(&cfg.Cluster.InsecureCA, flagManagementClusterInsecureCA, false,
		"Flag to indicate if the management cluster has an insecure CA that should be trusted")
	pflag.StringVar(&cfg.Cluster.Name, flagManagementClusterName, "",
		"The name of the management cluster.")
	pflag.StringVar(&cfg.Cluster.Pipeline, flagManagementClusterPipeline, "",
		"The pipeline of the management cluster.")
	pflag.StringVar(&cfg.Cluster.Region, flagManagementClusterRegion, "",
		"The region of the management cluster.")

	// Monitoring configuration flags
	pflag.BoolVar(&cfg.Monitoring.AlertmanagerEnabled, flagAlertmanagerEnabled, false,
		"Enable Alertmanager controller.")
	pflag.StringVar(&cfg.Monitoring.AlertmanagerSecretName, flagAlertmanagerSecretName, "",
		"The name of the secret containing the Alertmanager configuration.")
	pflag.StringVar(&cfg.Monitoring.AlertmanagerURL, flagAlertmanagerURL, "",
		"The URL of the Alertmanager API.")
	pflag.BoolVar(&cfg.Monitoring.Enabled, flagMonitoringEnabled, false,
		"Enable monitoring at the installation level.")
	pflag.Float64Var(&cfg.Monitoring.DefaultShardingStrategy.ScaleUpSeriesCount, flagMonitoringShardingScaleUpSeriesCount, 0,
		"Configures the number of time series needed to add an extra prometheus agent shard.")
	pflag.Float64Var(&cfg.Monitoring.DefaultShardingStrategy.ScaleDownPercentage, flagMonitoringShardingScaleDownPercentage, 0,
		"Configures the percentage of removed series to scale down the number of prometheus agent shards.")
	pflag.DurationVar(&cfg.Monitoring.WALTruncateFrequency, flagMonitoringWALTruncateFrequency, 2*time.Hour,
		"Configures how frequently the Write-Ahead Log (WAL) truncates segments.")
	pflag.StringVar(&cfg.Monitoring.MetricsQueryURL, flagMonitoringMetricsQueryURL, "http://mimir-gateway.mimir.svc/prometheus",
		"URL to query for cluster metrics")
	pflag.BoolVar(&cfg.Monitoring.NetworkEnabled, flagMonitoringNetworkEnabled, false,
		"Enable/disable network monitoring in Alloy logging configuration")

	// Queue configuration flags for Alloy remote write
	var queueBatchSendDeadline, queueMaxBackoff, queueMinBackoff, queueSampleAgeLimit string
	var queueCapacity, queueMaxSamplesPerSend, queueMaxShards, queueMinShards int
	var queueRetryOnHttp429 bool

	pflag.StringVar(&queueBatchSendDeadline, flagQueueBatchSendDeadline, "",
		"Maximum time samples wait in the buffer before sending (e.g., '5s'). If empty, Alloy default is used.")
	pflag.IntVar(&queueCapacity, flagQueueCapacity, 0,
		"Number of samples to buffer per shard. If 0, Alloy default is used.")
	pflag.StringVar(&queueMaxBackoff, flagQueueMaxBackoff, "",
		"Maximum backoff time between retries (e.g., '5m'). If empty, Alloy default is used.")
	pflag.IntVar(&queueMaxSamplesPerSend, flagQueueMaxSamplesPerSend, 0,
		"Maximum number of samples to send in a single request. If 0, Alloy default is used.")
	pflag.IntVar(&queueMaxShards, flagQueueMaxShards, 0,
		"Maximum number of shards to use. If 0, Alloy default is used.")
	pflag.StringVar(&queueMinBackoff, flagQueueMinBackoff, "",
		"Minimum backoff time between retries (e.g., '30ms'). If empty, Alloy default is used.")
	pflag.IntVar(&queueMinShards, flagQueueMinShards, 0,
		"Minimum number of shards to use. If 0, Alloy default is used.")
	pflag.BoolVar(&queueRetryOnHttp429, flagQueueRetryOnHttp429, false,
		"Retry when an HTTP 429 status code is received.")
	pflag.StringVar(&queueSampleAgeLimit, flagQueueSampleAgeLimit, "",
		"Maximum age of samples to send (e.g., '30m'). If empty, Alloy default is used.")

	// Tracing configuration flags
	pflag.BoolVar(&cfg.Tracing.Enabled, flagTracingEnabled, false,
		"Enable distributed tracing at the installation level.")

	// Logging configuration flags
	pflag.BoolVar(&cfg.Logging.Enabled, flagLoggingEnabled, false,
		"Enable logging at the installation level.")
	pflag.BoolVar(&cfg.Logging.EnableAlloyEventsReconciliation, flagLoggingEnableEvents, false,
		"Enable Alloy events reconciliation at the installation level.")
	pflag.StringSliceVar(&cfg.Logging.DefaultNamespaces, flagLoggingDefaultNamespaces, []string{},
		"Comma-separated list of namespaces to collect logs from by default on workload clusters")
	pflag.BoolVar(&cfg.Logging.EnableNodeFiltering, flagLoggingEnableNodeFiltering, false,
		"Enable/disable node filtering in Alloy logging configuration")
	pflag.StringSliceVar(&cfg.Logging.IncludeEventsNamespaces, flagLoggingIncludeEventsFromNamespaces, []string{},
		"Comma-separated list of namespaces to collect events from on workload clusters (if empty, collect from all)")
	pflag.StringSliceVar(&cfg.Logging.ExcludeEventsNamespaces, flagLoggingExcludeEventsFromNamespaces, []string{},
		"Comma-separated list of namespaces to exclude events from on workload clusters")

	// Zap logging options
	opts := zap.Options{
		Development: false,
	}
	opts.BindFlags(flag.CommandLine)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	// Parse Grafana URL after flags are parsed
	cfg.Grafana.URL, err = url.Parse(grafanaURL)
	if err != nil {
		return fmt.Errorf("failed to parse grafana URL: %w", err)
	}

	// Apply queue configuration flags after parsing (only if explicitly set)
	if pflag.CommandLine.Changed(flagQueueBatchSendDeadline) {
		cfg.Monitoring.QueueConfig.BatchSendDeadline = &queueBatchSendDeadline
	}
	if pflag.CommandLine.Changed(flagQueueCapacity) {
		cfg.Monitoring.QueueConfig.Capacity = &queueCapacity
	}
	if pflag.CommandLine.Changed(flagQueueMaxBackoff) {
		cfg.Monitoring.QueueConfig.MaxBackoff = &queueMaxBackoff
	}
	if pflag.CommandLine.Changed(flagQueueMaxSamplesPerSend) {
		cfg.Monitoring.QueueConfig.MaxSamplesPerSend = &queueMaxSamplesPerSend
	}
	if pflag.CommandLine.Changed(flagQueueMaxShards) {
		cfg.Monitoring.QueueConfig.MaxShards = &queueMaxShards
	}
	if pflag.CommandLine.Changed(flagQueueMinBackoff) {
		cfg.Monitoring.QueueConfig.MinBackoff = &queueMinBackoff
	}
	if pflag.CommandLine.Changed(flagQueueMinShards) {
		cfg.Monitoring.QueueConfig.MinShards = &queueMinShards
	}
	if pflag.CommandLine.Changed(flagQueueRetryOnHttp429) {
		cfg.Monitoring.QueueConfig.RetryOnHttp429 = &queueRetryOnHttp429
	}
	if pflag.CommandLine.Changed(flagQueueSampleAgeLimit) {
		cfg.Monitoring.QueueConfig.SampleAgeLimit = &queueSampleAgeLimit
	}

	return nil
}

// setupApplication sets up the application after configuration is complete.
func setupApplication() error {
	// Set up logging
	opts := zap.Options{
		Development: false,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Load environment variables
	_, err := env.UnmarshalFromEnviron(&cfg.Environment)
	if err != nil {
		return fmt.Errorf("failed to unmarshal environment variables: %w", err)
	}

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancelation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	tlsOpts := []func(*tls.Config){}
	if !cfg.Operator.EnableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: tlsOpts,
		CertDir: cfg.Operator.WebhookCertPath,
	})

	discardHelmSecretsSelector, err := labels.Parse("owner notin (helm,Helm)")
	if err != nil {
		return fmt.Errorf("failed to parse label selector: %w", err)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:   cfg.Operator.MetricsAddr,
			SecureServing: cfg.Operator.SecureMetrics,
			TLSOpts:       tlsOpts,
		},
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: cfg.Operator.ProbeAddr,
		LeaderElection:         cfg.Operator.EnableLeaderElection,
		LeaderElectionID:       "5c99b45b.giantswarm.io",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&v1.Secret{}: {
					// Do not cache any helm secrets to reduce memory usage.
					Label: discardHelmSecretsSelector,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("unable to start manager: %w", err)
	}

	// Initialize event recorder.
	record.InitFromRecorder(mgr.GetEventRecorderFor("observability-operator"))

	// Create Grafana client generator for dependency injection
	grafanaClientGen := &grafanaclient.DefaultGrafanaClientGenerator{}
	// Setup controller for the Cluster resource.
	err = controller.SetupClusterMonitoringReconciler(mgr, cfg)
	if err != nil {
		return fmt.Errorf("unable to create controller (ClusterMonitoringReconciler): %w", err)
	}

	// Setup controller for the GrafanaOrganization resource.
	err = controller.SetupGrafanaOrganizationReconciler(mgr, cfg, grafanaClientGen)
	if err != nil {
		return fmt.Errorf("unable to setup controller (GrafanaOrganizationReconciler): %w", err)
	}

	if cfg.Monitoring.AlertmanagerEnabled {
		// Setup controller for Alertmanager
		err = controller.SetupAlertmanagerReconciler(mgr, cfg)
		if err != nil {
			return fmt.Errorf("unable to setup controller (AlertmanagerReconciler): %w", err)
		}
	}

	// Setup controller for the Dashboard resource.
	err = controller.SetupDashboardReconciler(mgr, cfg, grafanaClientGen)
	if err != nil {
		return fmt.Errorf("unable to create controller (Dashboard): %w", err)
	}

	// nolint:goconst
	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err = webhookcorev1.SetupAlertmanagerConfigSecretWebhookWithManager(mgr); err != nil {
			return fmt.Errorf("unable to create webhook (Secret): %w", err)
		}
		if err = webhookcorev1.SetupDashboardConfigMapWebhookWithManager(mgr); err != nil {
			return fmt.Errorf("unable to create webhook (DashboardConfigMap): %w", err)
		}
		if err = webhookcorev1alpha1.SetupGrafanaOrganizationWebhookWithManager(mgr); err != nil {
			return fmt.Errorf("unable to create webhook (GrafanaOrganization v1alpha1): %w", err)
		}
		if err = webhookcorev1alpha2.SetupGrafanaOrganizationWebhookWithManager(mgr); err != nil {
			return fmt.Errorf("unable to create webhook (GrafanaOrganization v1alpha2): %w", err)
		}
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up ready check: %w", err)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		return fmt.Errorf("problem running manager: %w", err)
	}

	return nil
}
