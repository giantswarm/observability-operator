package client

import (
	grafana "github.com/grafana/grafana-openapi-client-go/client"
	"github.com/grafana/grafana-openapi-client-go/client/dashboards"
	"github.com/grafana/grafana-openapi-client-go/client/datasources"
	"github.com/grafana/grafana-openapi-client-go/client/orgs"
	"github.com/grafana/grafana-openapi-client-go/client/sso_settings"
)

// grafanaHTTPClient implements the GrafanaClient interface by wrapping
// the original GrafanaHTTPAPI and exposing all operations through client services.
type grafanaHTTPClient struct {
	api *grafana.GrafanaHTTPAPI
}

// NewGrafanaClient creates a new GrafanaClient implementation wrapping the provided GrafanaHTTPAPI
func NewGrafanaClient(api *grafana.GrafanaHTTPAPI) GrafanaClient {
	return &grafanaHTTPClient{
		api: api,
	}
}

// OrgID returns the current organization ID
func (g *grafanaHTTPClient) OrgID() int64 {
	return g.api.OrgID()
}

// WithOrgID sets the organization ID for subsequent requests and returns the client
func (g *grafanaHTTPClient) WithOrgID(orgID int64) GrafanaClient {
	g.api = g.api.WithOrgID(orgID)
	return g
}

// Datasources returns the datasources client service
func (g *grafanaHTTPClient) Datasources() datasources.ClientService {
	return g.api.Datasources
}

// Orgs returns the organizations client service
func (g *grafanaHTTPClient) Orgs() orgs.ClientService {
	return g.api.Orgs
}

// Dashboards returns the dashboards client service
func (g *grafanaHTTPClient) Dashboards() dashboards.ClientService {
	return g.api.Dashboards
}

// SsoSettings returns the SSO settings client service
func (g *grafanaHTTPClient) SsoSettings() sso_settings.ClientService {
	return g.api.SsoSettings
}
