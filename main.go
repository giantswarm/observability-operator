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

package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	appv1 "github.com/giantswarm/apiextensions-application/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/giantswarm/observability-operator/internal/controller"
	"github.com/giantswarm/observability-operator/pkg/bundle"
	"github.com/giantswarm/observability-operator/pkg/common"
	commonmonitoring "github.com/giantswarm/observability-operator/pkg/common/monitoring"
	"github.com/giantswarm/observability-operator/pkg/common/organization"
	"github.com/giantswarm/observability-operator/pkg/common/password"
	"github.com/giantswarm/observability-operator/pkg/grafana/client"
	"github.com/giantswarm/observability-operator/pkg/monitoring"
	"github.com/giantswarm/observability-operator/pkg/monitoring/alloy"
	"github.com/giantswarm/observability-operator/pkg/monitoring/heartbeat"
	"github.com/giantswarm/observability-operator/pkg/monitoring/mimir"
	"github.com/giantswarm/observability-operator/pkg/monitoring/prometheusagent"
	"github.com/giantswarm/observability-operator/pkg/monitoring/prometheusagent/sharding"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")

	metricsAddr          string
	enableLeaderElection bool
	probeAddr            string
	secureMetrics        bool
	enableHTTP2          bool

	managementClusterBaseDomain string
	managementClusterCustomer   string
	managementClusterInsecureCA bool
	managementClusterName       string
	managementClusterPipeline   string
	managementClusterRegion     string

	monitoringAgent                       string
	monitoringEnabled                     bool
	monitoringShardingScaleUpSeriesCount  float64
	monitoringShardingScaleDownPercentage float64
	monitoringWALTruncateFrequency        time.Duration
	prometheusVersion                     string
)

const (
	grafanaAdminUsernameEnvVar = "GRAFANA_ADMIN_USERNAME" // #nosec G101
	grafanaAdminPasswordEnvVar = "GRAFANA_ADMIN_PASSWORD" // #nosec G101
	grafanaTLSCertFileEnvVar   = "GRAFANA_TLS_CERT_FILE"  // #nosec G101
	grafanaTLSKeyFileEnvVar    = "GRAFANA_TLS_KEY_FILE"   // #nosec G101

	opsgenieApiKeyEnvVar = "OPSGENIE_API_KEY" // #nosec G101
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(appv1.AddToScheme(scheme))
	utilruntime.Must(observabilityv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080",
		"The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081",
		"The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", false,
		"If set the metrics endpoint is served securely")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.StringVar(&managementClusterBaseDomain, "management-cluster-base-domain", "",
		"The base domain of the management cluster.")
	flag.StringVar(&managementClusterCustomer, "management-cluster-customer", "",
		"The customer of the management cluster.")
	flag.BoolVar(&managementClusterInsecureCA, "management-cluster-insecure-ca", false,
		"Flag to indicate if the management cluster has an insecure CA that should be trusted")
	flag.StringVar(&managementClusterName, "management-cluster-name", "",
		"The name of the management cluster.")
	flag.StringVar(&managementClusterPipeline, "management-cluster-pipeline", "",
		"The pipeline of the management cluster.")
	flag.StringVar(&managementClusterRegion, "management-cluster-region", "",
		"The region of the management cluster.")
	// Monitoring configuration flags.
	flag.StringVar(&monitoringAgent, "monitoring-agent", commonmonitoring.MonitoringAgentAlloy,
		fmt.Sprintf("select monitoring agent to use (%s or %s)", commonmonitoring.MonitoringAgentPrometheus, commonmonitoring.MonitoringAgentAlloy))
	flag.BoolVar(&monitoringEnabled, "monitoring-enabled", false,
		"Enable monitoring at the management cluster level.")
	flag.Float64Var(&monitoringShardingScaleUpSeriesCount, "monitoring-sharding-scale-up-series-count", 0,
		"Configures the number of time series needed to add an extra prometheus agent shard.")
	flag.Float64Var(&monitoringShardingScaleDownPercentage, "monitoring-sharding-scale-down-percentage", 0,
		"Configures the percentage of removed series to scale down the number of prometheus agent shards.")
	flag.StringVar(&prometheusVersion, "prometheus-version", "",
		"The version of Prometheus Agents to deploy.")
	flag.DurationVar(&monitoringWALTruncateFrequency, "monitoring-wal-truncate-frequency", 2*time.Hour,
		"Configures how frequently the Write-Ahead Log (WAL) truncates segments.")
	opts := zap.Options{
		Development: false,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

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
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: tlsOpts,
	})

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:   metricsAddr,
			SecureServing: secureMetrics,
			TLSOpts:       tlsOpts,
		},
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
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
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Initialize event recorder.
	record.InitFromRecorder(mgr.GetEventRecorderFor("observability-operator"))

	var managementCluster common.ManagementCluster = common.ManagementCluster{
		BaseDomain: managementClusterBaseDomain,
		Customer:   managementClusterCustomer,
		InsecureCA: managementClusterInsecureCA,
		Name:       managementClusterName,
		Pipeline:   managementClusterPipeline,
		Region:     managementClusterRegion,
	}

	var opsgenieApiKey = os.Getenv(opsgenieApiKeyEnvVar)
	if opsgenieApiKey == "" {
		setupLog.Error(nil, fmt.Sprintf("environment variable %s not set", opsgenieApiKeyEnvVar))
		os.Exit(1)
	}

	heartbeatRepository, err := heartbeat.NewOpsgenieHeartbeatRepository(opsgenieApiKey, managementCluster)
	if err != nil {
		setupLog.Error(err, "unable to create heartbeat repository")
		os.Exit(1)
	}

	organizationRepository := organization.NewNamespaceRepository(mgr.GetClient())

	monitoringConfig := monitoring.Config{
		Enabled:         monitoringEnabled,
		MonitoringAgent: monitoringAgent,
		DefaultShardingStrategy: sharding.Strategy{
			ScaleUpSeriesCount:  monitoringShardingScaleUpSeriesCount,
			ScaleDownPercentage: monitoringShardingScaleDownPercentage,
		},
		WALTruncateFrequency: monitoringWALTruncateFrequency,
		PrometheusVersion:    prometheusVersion,
	}

	prometheusAgentService := prometheusagent.PrometheusAgentService{
		Client:                 mgr.GetClient(),
		OrganizationRepository: organizationRepository,
		PasswordManager:        password.SimpleManager{},
		ManagementCluster:      managementCluster,
		MonitoringConfig:       monitoringConfig,
	}

	alloyService := alloy.Service{
		Client:                 mgr.GetClient(),
		OrganizationRepository: organizationRepository,
		PasswordManager:        password.SimpleManager{},
		ManagementCluster:      managementCluster,
		MonitoringConfig:       monitoringConfig,
	}

	mimirService := mimir.MimirService{
		Client:            mgr.GetClient(),
		PasswordManager:   password.SimpleManager{},
		ManagementCluster: managementCluster,
	}

	if err = (&controller.ClusterMonitoringReconciler{
		Client:                     mgr.GetClient(),
		ManagementCluster:          managementCluster,
		HeartbeatRepository:        heartbeatRepository,
		PrometheusAgentService:     prometheusAgentService,
		AlloyService:               alloyService,
		MimirService:               mimirService,
		MonitoringConfig:           monitoringConfig,
		BundleConfigurationService: bundle.NewBundleConfigurationService(mgr.GetClient(), monitoringConfig),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Cluster")
		os.Exit(1)
	}

	// Generate Grafana client
	// Get grafana admin-password and admin-user
	grafanaAdminCredentials := client.AdminCredentials{
		Username: os.Getenv(grafanaAdminUsernameEnvVar),
		Password: os.Getenv(grafanaAdminPasswordEnvVar),
	}
	if grafanaAdminCredentials.Username == "" {
		setupLog.Error(nil, fmt.Sprintf("environment variable %s not set", grafanaAdminUsernameEnvVar))
		os.Exit(1)
	}
	if grafanaAdminCredentials.Password == "" {
		setupLog.Error(nil, fmt.Sprintf("environment variable %s not set", grafanaAdminPasswordEnvVar))
		os.Exit(1)
	}
	grafanaTLSConfig := client.TLSConfig{
		Cert: os.Getenv(grafanaTLSCertFileEnvVar),
		Key:  os.Getenv(grafanaTLSKeyFileEnvVar),
	}
	grafanaAPI, err := client.GenerateGrafanaClient(grafanaAdminCredentials, grafanaTLSConfig)
	if err != nil {
		setupLog.Error(err, "unable to create grafana client")
		os.Exit(1)
	}

	if err = (&controller.GrafanaOrganizationReconciler{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		GrafanaAPI: grafanaAPI,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "GrafanaOrganization")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
