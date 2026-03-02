package grafana

import (
	"github.com/giantswarm/observability-operator/pkg/config"
	grafanaClient "github.com/giantswarm/observability-operator/pkg/grafana/client"
)

type Service struct {
	grafanaClient grafanaClient.GrafanaClient
	cfg           config.Config
}

func NewService(grafanaClient grafanaClient.GrafanaClient, cfg config.Config) *Service {
	return &Service{
		grafanaClient: grafanaClient,
		cfg:           cfg,
	}
}
