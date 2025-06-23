package client

import (
	"github.com/grafana/grafana-openapi-client-go/client/dashboards"
	"github.com/grafana/grafana-openapi-client-go/client/datasources"
	"github.com/grafana/grafana-openapi-client-go/client/orgs"
	"github.com/grafana/grafana-openapi-client-go/client/sso_settings"
	"github.com/grafana/grafana-openapi-client-go/models"
)

// GrafanaClient defines the unified interface that merges all Grafana API operations
// This interface allows for easier testing by providing a single point
// to mock all Grafana API operations.
type GrafanaClient interface {
	// Organization context management
	OrgID() int64
	WithOrgID(orgID int64) GrafanaClient

	// Datasources operations
	AddDataSource(body *models.AddDataSourceCommand) (*datasources.AddDataSourceOK, error)
	UpdateDataSourceByUID(uid string, body *models.UpdateDataSourceCommand) (*datasources.UpdateDataSourceByUIDOK, error)
	DeleteDataSourceByUID(uid string) (*datasources.DeleteDataSourceByUIDOK, error)
	GetDataSources() (*datasources.GetDataSourcesOK, error)

	// Organizations operations
	CreateOrg(body *models.CreateOrgCommand) (*orgs.CreateOrgOK, error)
	UpdateOrg(orgID int64, body *models.UpdateOrgForm) (*orgs.UpdateOrgOK, error)
	DeleteOrgByID(orgID int64) (*orgs.DeleteOrgByIDOK, error)
	GetOrgByName(orgName string) (*orgs.GetOrgByNameOK, error)
	GetOrgByID(orgID int64) (*orgs.GetOrgByIDOK, error)

	// Dashboards operations
	PostDashboard(body *models.SaveDashboardCommand) (*dashboards.PostDashboardOK, error)
	GetDashboardByUID(uid string) (*dashboards.GetDashboardByUIDOK, error)
	DeleteDashboardByUID(uid string) (*dashboards.DeleteDashboardByUIDOK, error)

	// SSO Settings operations
	GetProviderSettings(provider string) (*sso_settings.GetProviderSettingsOK, error)
	UpdateProviderSettings(provider string, body *models.UpdateProviderSettingsParamsBody) (*sso_settings.UpdateProviderSettingsNoContent, error)
}
