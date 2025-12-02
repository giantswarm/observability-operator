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
	"github.com/giantswarm/observability-operator/internal/controller"
	webhookcorev1 "github.com/giantswarm/observability-operator/internal/webhook/v1"
	webhookcorev1alpha1 "github.com/giantswarm/observability-operator/internal/webhook/v1alpha1"
	"github.com/giantswarm/observability-operator/pkg/config"
	grafanaclient "github.com/giantswarm/observability-operator/pkg/grafana/client"
	//+kubebuilder:scaffold:imports
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
	flag.StringVar(&cfg.Operator.MetricsAddr, "metrics-bind-address", ":8080",
		"The address the metric endpoint binds to.")
	flag.StringVar(&cfg.Operator.ProbeAddr, "health-probe-bind-address", ":8081",
		"The address the probe endpoint binds to.")
	flag.BoolVar(&cfg.Operator.EnableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&cfg.Operator.SecureMetrics, "metrics-secure", false,
		"If set the metrics endpoint is served securely")
	flag.BoolVar(&cfg.Operator.EnableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.StringVar(&cfg.Operator.WebhookCertPath, "webhook-cert-path", "/tmp/k8s-webhook-server/serving-certs",
		"Path to the directory where the webhook server will store its TLS certificate and key.")
	flag.StringVar(&cfg.Operator.OperatorNamespace, "operator-namespace", "",
		"The namespace where the observability-operator is running.")

	// Grafana configuration flags
	var grafanaURL string
	flag.StringVar(&grafanaURL, "grafana-url", "http://grafana.monitoring.svc.cluster.local", "grafana URL")
	// Parse Grafana URL
	cfg.Grafana.URL, err = url.Parse(grafanaURL)
	if err != nil {
		return fmt.Errorf("failed to parse grafana URL: %w", err)
	}

	// Management cluster configuration flags
	flag.StringVar(&cfg.Cluster.BaseDomain, "management-cluster-base-domain", "",
		"The base domain of the management cluster.")
	flag.StringVar(&cfg.Cluster.Customer, "management-cluster-customer", "",
		"The customer of the management cluster.")
	flag.BoolVar(&cfg.Cluster.InsecureCA, "management-cluster-insecure-ca", false,
		"Flag to indicate if the management cluster has an insecure CA that should be trusted")
	flag.StringVar(&cfg.Cluster.Name, "management-cluster-name", "",
		"The name of the management cluster.")
	flag.StringVar(&cfg.Cluster.Pipeline, "management-cluster-pipeline", "",
		"The pipeline of the management cluster.")
	flag.StringVar(&cfg.Cluster.Region, "management-cluster-region", "",
		"The region of the management cluster.")

	// Monitoring configuration flags
	flag.BoolVar(&cfg.Monitoring.AlertmanagerEnabled, "alertmanager-enabled", false,
		"Enable Alertmanager controller.")
	flag.StringVar(&cfg.Monitoring.AlertmanagerSecretName, "alertmanager-secret-name", "",
		"The name of the secret containing the Alertmanager configuration.")
	flag.StringVar(&cfg.Monitoring.AlertmanagerURL, "alertmanager-url", "",
		"The URL of the Alertmanager API.")
	flag.BoolVar(&cfg.Monitoring.Enabled, "monitoring-enabled", false,
		"Enable monitoring at the management cluster level.")
	flag.Float64Var(&cfg.Monitoring.DefaultShardingStrategy.ScaleUpSeriesCount, "monitoring-sharding-scale-up-series-count", 0,
		"Configures the number of time series needed to add an extra prometheus agent shard.")
	flag.Float64Var(&cfg.Monitoring.DefaultShardingStrategy.ScaleDownPercentage, "monitoring-sharding-scale-down-percentage", 0,
		"Configures the percentage of removed series to scale down the number of prometheus agent shards.")
	flag.DurationVar(&cfg.Monitoring.WALTruncateFrequency, "monitoring-wal-truncate-frequency", 2*time.Hour,
		"Configures how frequently the Write-Ahead Log (WAL) truncates segments.")
	flag.StringVar(&cfg.Monitoring.MetricsQueryURL, "monitoring-metrics-query-url", "http://mimir-gateway.mimir.svc/prometheus",
		"URL to query for cluster metrics")

	// Queue configuration flags for Alloy remote write
	var queueBatchSendDeadline, queueMaxBackoff, queueMinBackoff, queueSampleAgeLimit string
	var queueCapacity, queueMaxSamplesPerSend, queueMaxShards, queueMinShards int
	var queueRetryOnHttp429 bool

	flag.StringVar(&queueBatchSendDeadline, "monitoring-queue-config-batch-send-deadline", "",
		"Maximum time samples wait in the buffer before sending (e.g., '5s'). If empty, Alloy default is used.")
	flag.IntVar(&queueCapacity, "monitoring-queue-config-capacity", 0,
		"Number of samples to buffer per shard. If 0, Alloy default is used.")
	flag.StringVar(&queueMaxBackoff, "monitoring-queue-config-max-backoff", "",
		"Maximum backoff time between retries (e.g., '5m'). If empty, Alloy default is used.")
	flag.IntVar(&queueMaxSamplesPerSend, "monitoring-queue-config-max-samples-per-send", 0,
		"Maximum number of samples to send in a single request. If 0, Alloy default is used.")
	flag.IntVar(&queueMaxShards, "monitoring-queue-config-max-shards", 0,
		"Maximum number of shards to use. If 0, Alloy default is used.")
	flag.StringVar(&queueMinBackoff, "monitoring-queue-config-min-backoff", "",
		"Minimum backoff time between retries (e.g., '30ms'). If empty, Alloy default is used.")
	flag.IntVar(&queueMinShards, "monitoring-queue-config-min-shards", 0,
		"Minimum number of shards to use. If 0, Alloy default is used.")
	flag.BoolVar(&queueRetryOnHttp429, "monitoring-queue-config-retry-on-http-429", false,
		"Retry when an HTTP 429 status code is received.")
	flag.StringVar(&queueSampleAgeLimit, "monitoring-queue-config-sample-age-limit", "",
		"Maximum age of samples to send (e.g., '30m'). If empty, Alloy default is used.")

	// Tracing configuration flags
	flag.BoolVar(&cfg.Tracing.Enabled, "tracing-enabled", false,
		"Enable distributed tracing support in Grafana.")

	// Logging configuration flags
	flag.BoolVar(&cfg.Logging.Enabled, "logging-enabled", false,
		"Enable logging support in Grafana.")

	// Zap logging options
	opts := zap.Options{
		Development: false,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	// Apply queue configuration flags after parsing
	if queueBatchSendDeadline != "" {
		cfg.Monitoring.QueueConfig.BatchSendDeadline = &queueBatchSendDeadline
	}
	if queueCapacity > 0 {
		cfg.Monitoring.QueueConfig.Capacity = &queueCapacity
	}
	if queueMaxBackoff != "" {
		cfg.Monitoring.QueueConfig.MaxBackoff = &queueMaxBackoff
	}
	if queueMaxSamplesPerSend > 0 {
		cfg.Monitoring.QueueConfig.MaxSamplesPerSend = &queueMaxSamplesPerSend
	}
	if queueMaxShards > 0 {
		cfg.Monitoring.QueueConfig.MaxShards = &queueMaxShards
	}
	if queueMinBackoff != "" {
		cfg.Monitoring.QueueConfig.MinBackoff = &queueMinBackoff
	}
	if queueMinShards > 0 {
		cfg.Monitoring.QueueConfig.MinShards = &queueMinShards
	}
	if queueRetryOnHttp429 {
		cfg.Monitoring.QueueConfig.RetryOnHttp429 = &queueRetryOnHttp429
	}
	if queueSampleAgeLimit != "" {
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
			return fmt.Errorf("unable to create webhook (GrafanaOrganization): %w", err)
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
