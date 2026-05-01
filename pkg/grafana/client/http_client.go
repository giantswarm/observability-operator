package client

import (
	grafana "github.com/grafana/grafana-openapi-client-go/client"
	"github.com/grafana/grafana-openapi-client-go/client/dashboards"
	"github.com/grafana/grafana-openapi-client-go/client/datasources"
	"github.com/grafana/grafana-openapi-client-go/client/folders"
	"github.com/grafana/grafana-openapi-client-go/client/orgs"
	"github.com/grafana/grafana-openapi-client-go/client/sso_settings"
)

// grafanaHTTPClient adapts the upstream GrafanaHTTPAPI to the GrafanaClient interface.
type grafanaHTTPClient struct {
	api *grafana.GrafanaHTTPAPI
}

// NewGrafanaClient creates a new GrafanaClient implementation wrapping the provided GrafanaHTTPAPI
func NewGrafanaClient(api *grafana.GrafanaHTTPAPI) GrafanaClient {
	return &grafanaHTTPClient{api: api}
}

// WithOrgID returns a new GrafanaClient scoped to orgID. The receiver is not modified.
func (g *grafanaHTTPClient) WithOrgID(orgID int64) GrafanaClient {
	return &grafanaHTTPClient{api: g.api.WithOrgID(orgID)}
}

func (g *grafanaHTTPClient) Datasources() datasources.ClientService {
	return g.api.Datasources
}

func (g *grafanaHTTPClient) Orgs() orgs.ClientService {
	return g.api.Orgs
}

func (g *grafanaHTTPClient) Dashboards() dashboards.ClientService {
	return g.api.Dashboards
}

func (g *grafanaHTTPClient) Folders() folders.ClientService {
	return g.api.Folders
}

func (g *grafanaHTTPClient) SsoSettings() sso_settings.ClientService {
	return g.api.SsoSettings
}
