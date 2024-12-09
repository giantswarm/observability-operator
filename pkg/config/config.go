package config

import (
	"github.com/giantswarm/observability-operator/pkg/common"
	"github.com/giantswarm/observability-operator/pkg/monitoring"
)

type Config struct {
	MetricsAddr          string
	EnableLeaderElection bool
	ProbeAddr            string
	SecureMetrics        bool
	EnableHTTP2          bool

	AlertmanagerSecretName string
	AlertmanagerURL        string
	Namespace              string

	ManagementCluster common.ManagementCluster

	Monitoring monitoring.Config
}
