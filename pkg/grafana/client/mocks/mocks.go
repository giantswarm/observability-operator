package mocks

import (
	"context"
	"net/url"

	"github.com/grafana/grafana-openapi-client-go/client/dashboards"
	"github.com/grafana/grafana-openapi-client-go/client/datasources"
	"github.com/grafana/grafana-openapi-client-go/client/orgs"
	"github.com/grafana/grafana-openapi-client-go/client/sso_settings"
	"github.com/grafana/grafana-openapi-client-go/models"
	"github.com/stretchr/testify/mock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	grafanaclient "github.com/giantswarm/observability-operator/pkg/grafana/client"
)

// MockGrafanaClient is a testify mock for the GrafanaClient interface
type MockGrafanaClient struct {
	mock.Mock
}

func (m *MockGrafanaClient) OrgID() int64 {
	args := m.Called()
	return args.Get(0).(int64)
}

func (m *MockGrafanaClient) WithOrgID(orgID int64) grafanaclient.GrafanaClient {
	args := m.Called(orgID)
	return args.Get(0).(grafanaclient.GrafanaClient)
}

func (m *MockGrafanaClient) AddDataSource(body *models.AddDataSourceCommand) (*datasources.AddDataSourceOK, error) {
	args := m.Called(body)
	return args.Get(0).(*datasources.AddDataSourceOK), args.Error(1)
}

func (m *MockGrafanaClient) UpdateDataSourceByUID(uid string, body *models.UpdateDataSourceCommand) (*datasources.UpdateDataSourceByUIDOK, error) {
	args := m.Called(uid, body)
	return args.Get(0).(*datasources.UpdateDataSourceByUIDOK), args.Error(1)
}

func (m *MockGrafanaClient) DeleteDataSourceByUID(uid string) (*datasources.DeleteDataSourceByUIDOK, error) {
	args := m.Called(uid)
	return args.Get(0).(*datasources.DeleteDataSourceByUIDOK), args.Error(1)
}

func (m *MockGrafanaClient) GetDataSources() (*datasources.GetDataSourcesOK, error) {
	args := m.Called()
	return args.Get(0).(*datasources.GetDataSourcesOK), args.Error(1)
}

func (m *MockGrafanaClient) CreateOrg(body *models.CreateOrgCommand) (*orgs.CreateOrgOK, error) {
	args := m.Called(body)
	return args.Get(0).(*orgs.CreateOrgOK), args.Error(1)
}

func (m *MockGrafanaClient) UpdateOrg(orgID int64, body *models.UpdateOrgForm) (*orgs.UpdateOrgOK, error) {
	args := m.Called(orgID, body)
	return args.Get(0).(*orgs.UpdateOrgOK), args.Error(1)
}

func (m *MockGrafanaClient) DeleteOrgByID(orgID int64) (*orgs.DeleteOrgByIDOK, error) {
	args := m.Called(orgID)
	return args.Get(0).(*orgs.DeleteOrgByIDOK), args.Error(1)
}

func (m *MockGrafanaClient) GetOrgByName(orgName string) (*orgs.GetOrgByNameOK, error) {
	args := m.Called(orgName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*orgs.GetOrgByNameOK), args.Error(1)
}

func (m *MockGrafanaClient) GetOrgByID(orgID int64) (*orgs.GetOrgByIDOK, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*orgs.GetOrgByIDOK), args.Error(1)
}

func (m *MockGrafanaClient) PostDashboard(body *models.SaveDashboardCommand) (*dashboards.PostDashboardOK, error) {
	args := m.Called(body)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dashboards.PostDashboardOK), args.Error(1)
}

func (m *MockGrafanaClient) GetDashboardByUID(uid string) (*dashboards.GetDashboardByUIDOK, error) {
	args := m.Called(uid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dashboards.GetDashboardByUIDOK), args.Error(1)
}

func (m *MockGrafanaClient) DeleteDashboardByUID(uid string) (*dashboards.DeleteDashboardByUIDOK, error) {
	args := m.Called(uid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dashboards.DeleteDashboardByUIDOK), args.Error(1)
}

func (m *MockGrafanaClient) GetProviderSettings(provider string) (*sso_settings.GetProviderSettingsOK, error) {
	args := m.Called(provider)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sso_settings.GetProviderSettingsOK), args.Error(1)
}

func (m *MockGrafanaClient) UpdateProviderSettings(provider string, body *models.UpdateProviderSettingsParamsBody) (*sso_settings.UpdateProviderSettingsNoContent, error) {
	args := m.Called(provider, body)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sso_settings.UpdateProviderSettingsNoContent), args.Error(1)
}

// MockGrafanaClientGenerator is a testify mock for the GrafanaClientGenerator interface
type MockGrafanaClientGenerator struct {
	mock.Mock
}

func (m *MockGrafanaClientGenerator) GenerateGrafanaClient(ctx context.Context, k8sClient client.Client, grafanaURL *url.URL) (grafanaclient.GrafanaClient, error) {
	args := m.Called(ctx, k8sClient, grafanaURL)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(grafanaclient.GrafanaClient), args.Error(1)
}

// Ensure mock implementations comply with their interfaces
var _ grafanaclient.GrafanaClient = (*MockGrafanaClient)(nil)
var _ grafanaclient.GrafanaClientGenerator = (*MockGrafanaClientGenerator)(nil)
