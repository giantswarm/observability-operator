package grafana

import (
	"github.com/giantswarm/observability-operator/pkg/config"
	"github.com/giantswarm/observability-operator/pkg/domain/organization"
	grafanaClient "github.com/giantswarm/observability-operator/pkg/grafana/client"
)

type Service struct {
	grafanaClient grafanaClient.GrafanaClient
	cfg           config.Config

	// orgsByName memoizes organization lookups by name for the lifetime of the
	// Service (a single reconcile), so repeated FindOrgByName calls — e.g. once
	// per dashboard in a ConfigMap — hit the Grafana API only once per organization.
	orgsByName map[string]*organization.Organization
}

func NewService(grafanaClient grafanaClient.GrafanaClient, cfg config.Config) *Service {
	return &Service{
		grafanaClient: grafanaClient,
		cfg:           cfg,
		orgsByName:    make(map[string]*organization.Organization),
	}
}
