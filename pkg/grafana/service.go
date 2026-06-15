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

	// foldersByPath memoizes the leaf folder UID for a folder path already ensured
	// during the Service lifetime. All dashboards in a ConfigMap share the same
	// folder path, so the hierarchy is reconciled with Grafana only once per path.
	foldersByPath map[string]string
}

func NewService(grafanaClient grafanaClient.GrafanaClient, cfg config.Config) *Service {
	return &Service{
		grafanaClient: grafanaClient,
		cfg:           cfg,
		orgsByName:    make(map[string]*organization.Organization),
		foldersByPath: make(map[string]string),
	}
}
