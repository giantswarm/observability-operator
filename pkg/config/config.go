package config

import (
	"net/url"

	"github.com/giantswarm/observability-operator/pkg/common"
	"github.com/giantswarm/observability-operator/pkg/monitoring"
	"github.com/giantswarm/observability-operator/pkg/tracing"
)

type Config struct {
	MetricsAddr          string
	EnableLeaderElection bool
	ProbeAddr            string
	SecureMetrics        bool
	EnableHTTP2          bool
	WebhookCertPath      string
	OperatorNamespace    string
	GrafanaURL           *url.URL

	ManagementCluster common.ManagementCluster

	Monitoring monitoring.Config
	Tracing    tracing.Config

	Environment Environment
}

type Environment struct {
	OpsgenieApiKey string `env:"OPSGENIE_API_KEY,required=true"`
}
