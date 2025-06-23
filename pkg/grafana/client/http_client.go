package client

import (
	grafana "github.com/grafana/grafana-openapi-client-go/client"
	"github.com/grafana/grafana-openapi-client-go/client/dashboards"
	"github.com/grafana/grafana-openapi-client-go/client/datasources"
	"github.com/grafana/grafana-openapi-client-go/client/orgs"
	"github.com/grafana/grafana-openapi-client-go/client/sso_settings"
	"github.com/grafana/grafana-openapi-client-go/models"
)

// grafanaHTTPClient implements the GrafanaClient interface by wrapping
// the original GrafanaHTTPAPI and exposing all operations through a unified interface.
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

// Datasources operations
func (g *grafanaHTTPClient) AddDataSource(body *models.AddDataSourceCommand) (*datasources.AddDataSourceOK, error) {
	return g.api.Datasources.AddDataSource(body)
}

func (g *grafanaHTTPClient) UpdateDataSourceByUID(uid string, body *models.UpdateDataSourceCommand) (*datasources.UpdateDataSourceByUIDOK, error) {
	return g.api.Datasources.UpdateDataSourceByUID(uid, body)
}

func (g *grafanaHTTPClient) DeleteDataSourceByUID(uid string) (*datasources.DeleteDataSourceByUIDOK, error) {
	return g.api.Datasources.DeleteDataSourceByUID(uid)
}

func (g *grafanaHTTPClient) GetDataSources() (*datasources.GetDataSourcesOK, error) {
	return g.api.Datasources.GetDataSources()
}

// Organizations operations
func (g *grafanaHTTPClient) CreateOrg(body *models.CreateOrgCommand) (*orgs.CreateOrgOK, error) {
	return g.api.Orgs.CreateOrg(body)
}

func (g *grafanaHTTPClient) UpdateOrg(orgID int64, body *models.UpdateOrgForm) (*orgs.UpdateOrgOK, error) {
	return g.api.Orgs.UpdateOrg(orgID, body)
}

func (g *grafanaHTTPClient) DeleteOrgByID(orgID int64) (*orgs.DeleteOrgByIDOK, error) {
	return g.api.Orgs.DeleteOrgByID(orgID)
}

func (g *grafanaHTTPClient) GetOrgByName(orgName string) (*orgs.GetOrgByNameOK, error) {
	return g.api.Orgs.GetOrgByName(orgName)
}

func (g *grafanaHTTPClient) GetOrgByID(orgID int64) (*orgs.GetOrgByIDOK, error) {
	return g.api.Orgs.GetOrgByID(orgID)
}

// Dashboards operations
func (g *grafanaHTTPClient) PostDashboard(body *models.SaveDashboardCommand) (*dashboards.PostDashboardOK, error) {
	return g.api.Dashboards.PostDashboard(body)
}

func (g *grafanaHTTPClient) GetDashboardByUID(uid string) (*dashboards.GetDashboardByUIDOK, error) {
	return g.api.Dashboards.GetDashboardByUID(uid)
}

func (g *grafanaHTTPClient) DeleteDashboardByUID(uid string) (*dashboards.DeleteDashboardByUIDOK, error) {
	return g.api.Dashboards.DeleteDashboardByUID(uid)
}

// SSO Settings operations
func (g *grafanaHTTPClient) GetProviderSettings(provider string) (*sso_settings.GetProviderSettingsOK, error) {
	return g.api.SsoSettings.GetProviderSettings(provider)
}

func (g *grafanaHTTPClient) UpdateProviderSettings(provider string, body *models.UpdateProviderSettingsParamsBody) (*sso_settings.UpdateProviderSettingsNoContent, error) {
	return g.api.SsoSettings.UpdateProviderSettings(provider, body)
}
