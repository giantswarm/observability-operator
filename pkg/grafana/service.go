package grafana

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	grafanaClient "github.com/giantswarm/observability-operator/pkg/grafana/client"
)

type Service struct {
	client        client.Client
	grafanaClient grafanaClient.GrafanaClient
}

func NewService(k8sClient client.Client, grafanaClient grafanaClient.GrafanaClient) *Service {
	s := &Service{
		client:        k8sClient,
		grafanaClient: grafanaClient,
	}

	return s
}
