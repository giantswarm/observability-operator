package grafana

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/observability-operator/pkg/config"
	grafanaClient "github.com/giantswarm/observability-operator/pkg/grafana/client"
)

type Service struct {
	client        client.Client
	grafanaClient grafanaClient.GrafanaClient
	cfg           config.Config
}

func NewService(k8sClient client.Client, grafanaClient grafanaClient.GrafanaClient, cfg config.Config) *Service {
	s := &Service{
		client:        k8sClient,
		grafanaClient: grafanaClient,
		cfg:           cfg,
	}

	return s
}
