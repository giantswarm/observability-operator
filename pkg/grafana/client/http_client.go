package client

import (
	grafana "github.com/grafana/grafana-openapi-client-go/client"
	"github.com/grafana/grafana-openapi-client-go/client/dashboards"
	"github.com/grafana/grafana-openapi-client-go/client/datasources"
	"github.com/grafana/grafana-openapi-client-go/client/folders"
	"github.com/grafana/grafana-openapi-client-go/client/orgs"
	"github.com/grafana/grafana-openapi-client-go/client/sso_settings"
)

// grafanaHTTPClient adapts the upstream GrafanaHTTPAPI and the app-platform
// dashboard API client to the GrafanaClient interface.
type grafanaHTTPClient struct {
	api          *grafana.GrafanaHTTPAPI
	dashboardAPI *dashboardAPIClient
}

// NewGrafanaClient creates a new GrafanaClient implementation wrapping the provided
// GrafanaHTTPAPI and app-platform dashboard API client.
func NewGrafanaClient(api *grafana.GrafanaHTTPAPI, dashboardAPI *dashboardAPIClient) GrafanaClient {
	return &grafanaHTTPClient{api: api, dashboardAPI: dashboardAPI}
}

// WithOrgID returns a new GrafanaClient scoped to orgID. The receiver is not modified.
func (g *grafanaHTTPClient) WithOrgID(orgID int64) GrafanaClient {
	return &grafanaHTTPClient{api: g.api.WithOrgID(orgID), dashboardAPI: g.dashboardAPI.withOrgID(orgID)}
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

func (g *grafanaHTTPClient) DashboardAPI() DashboardAPIClient {
	return g.dashboardAPI
}
