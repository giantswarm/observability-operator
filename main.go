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

	"github.com/Netflix/go-env"
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
	commonmonitoring "github.com/giantswarm/observability-operator/pkg/common/monitoring"
	"github.com/giantswarm/observability-operator/pkg/config"
	//+kubebuilder:scaffold:imports
)

var (
	conf config.Config

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
	flag.StringVar(&conf.MetricsAddr, "metrics-bind-address", ":8080",
		"The address the metric endpoint binds to.")
	flag.StringVar(&conf.ProbeAddr, "health-probe-bind-address", ":8081",
		"The address the probe endpoint binds to.")
	flag.BoolVar(&conf.EnableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&conf.SecureMetrics, "metrics-secure", false,
		"If set the metrics endpoint is served securely")
	flag.BoolVar(&conf.EnableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.StringVar(&conf.OperatorNamespace, "operator-namespace", "",
		"The namespace where the observability-operator is running.")

	// Management cluster configuration flags.
	flag.StringVar(&conf.ManagementCluster.BaseDomain, "management-cluster-base-domain", "",
		"The base domain of the management cluster.")
	flag.StringVar(&conf.ManagementCluster.Customer, "management-cluster-customer", "",
		"The customer of the management cluster.")
	flag.BoolVar(&conf.ManagementCluster.InsecureCA, "management-cluster-insecure-ca", false,
		"Flag to indicate if the management cluster has an insecure CA that should be trusted")
	flag.StringVar(&conf.ManagementCluster.Name, "management-cluster-name", "",
		"The name of the management cluster.")
	flag.StringVar(&conf.ManagementCluster.Pipeline, "management-cluster-pipeline", "",
		"The pipeline of the management cluster.")
	flag.StringVar(&conf.ManagementCluster.Region, "management-cluster-region", "",
		"The region of the management cluster.")

	// Monitoring configuration flags.
	flag.BoolVar(&conf.Monitoring.AlertmanagerEnabled, "alertmanager-enabled", false,
		"Enable Alertmanager controller.")
	flag.StringVar(&conf.Monitoring.AlertmanagerSecretName, "alertmanager-secret-name", "",
		"The name of the secret containing the Alertmanager configuration.")
	flag.StringVar(&conf.Monitoring.AlertmanagerURL, "alertmanager-url", "",
		"The URL of the Alertmanager API.")
	flag.StringVar(&conf.Monitoring.MonitoringAgent, "monitoring-agent", commonmonitoring.MonitoringAgentAlloy,
		fmt.Sprintf("select monitoring agent to use (%s or %s)", commonmonitoring.MonitoringAgentPrometheus, commonmonitoring.MonitoringAgentAlloy))
	flag.BoolVar(&conf.Monitoring.Enabled, "monitoring-enabled", false,
		"Enable monitoring at the management cluster level.")
	flag.Float64Var(&conf.Monitoring.DefaultShardingStrategy.ScaleUpSeriesCount, "monitoring-sharding-scale-up-series-count", 0,
		"Configures the number of time series needed to add an extra prometheus agent shard.")
	flag.Float64Var(&conf.Monitoring.DefaultShardingStrategy.ScaleDownPercentage, "monitoring-sharding-scale-down-percentage", 0,
		"Configures the percentage of removed series to scale down the number of prometheus agent shards.")
	flag.StringVar(&conf.Monitoring.PrometheusVersion, "prometheus-version", "",
		"The version of Prometheus Agents to deploy.")
	flag.DurationVar(&conf.Monitoring.WALTruncateFrequency, "monitoring-wal-truncate-frequency", 2*time.Hour,
		"Configures how frequently the Write-Ahead Log (WAL) truncates segments.")
	flag.StringVar(&conf.Monitoring.MetricsQueryURL, "metrics-query-url", "http://mimir-gateway.mimir.svc/prometheus",
		"URL to query for cluster metrics")
	opts := zap.Options{
		Development: false,
	}

	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Load environment variables.
	_, err := env.UnmarshalFromEnviron(&conf.Environment)
	if err != nil {
		setupLog.Error(err, "failed to unmarshal environment variables")
		os.Exit(1)
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
	if !conf.EnableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: tlsOpts,
	})

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:   conf.MetricsAddr,
			SecureServing: conf.SecureMetrics,
			TLSOpts:       tlsOpts,
		},
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: conf.ProbeAddr,
		LeaderElection:         conf.EnableLeaderElection,
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

	// Setup controller for the Cluster resource.
	err = controller.SetupClusterMonitoringReconciler(mgr, conf)
	if err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterMonitoringReconciler")
		os.Exit(1)
	}

	// Setup controller for the GrafanaOrganization resource.
	err = controller.SetupGrafanaOrganizationReconciler(mgr, conf.Environment)
	if err != nil {
		setupLog.Error(err, "unable to setup controller", "controller", "GrafanaOrganizationReconciler")
		os.Exit(1)
	}

	if conf.Monitoring.AlertmanagerEnabled {
		// Setup controller for Alertmanager
		err = controller.SetupAlertmanagerReconciler(mgr, conf)
		if err != nil {
			setupLog.Error(err, "unable to setup controller", "controller", "AlertmanagerReconciler")
			os.Exit(1)
		}
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
