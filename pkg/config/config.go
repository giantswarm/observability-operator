package config

import (
	"net/url"

	"github.com/giantswarm/observability-operator/pkg/common"
	"github.com/giantswarm/observability-operator/pkg/monitoring"
)

type Config struct {
	MetricsAddr          string
	EnableLeaderElection bool
	ProbeAddr            string
	SecureMetrics        bool
	EnableHTTP2          bool
	OperatorNamespace    string
	GrafanaURL           *url.URL

	ManagementCluster common.ManagementCluster

	Monitoring monitoring.Config

	Environment Environment
}

type Environment struct {
	GrafanaAdminUsername string `env:"GRAFANA_ADMIN_USERNAME,required=true"`
	GrafanaAdminPassword string `env:"GRAFANA_ADMIN_PASSWORD,required=true"`
	GrafanaTLSCertFile   string `env:"GRAFANA_TLS_CERT_FILE,required=true"`
	GrafanaTLSKeyFile    string `env:"GRAFANA_TLS_KEY_FILE,required=true"`

	OpsgenieApiKey string `env:"OPSGENIE_API_KEY,required=true"`
}
