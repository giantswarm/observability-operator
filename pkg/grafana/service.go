package grafana

import (
	grafanaAPI "github.com/grafana/grafana-openapi-client-go/client"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Service struct {
	client     client.Client
	grafanaAPI *grafanaAPI.GrafanaHTTPAPI
}

func NewService(grafanaAPI *grafanaAPI.GrafanaHTTPAPI) *Service {
	s := &Service{
		grafanaAPI: grafanaAPI,
	}

	return s
}
