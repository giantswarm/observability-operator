package mocks

import (
	"github.com/go-openapi/runtime"
	"github.com/grafana/grafana-openapi-client-go/client/dashboards"
	"github.com/grafana/grafana-openapi-client-go/client/datasources"
	"github.com/grafana/grafana-openapi-client-go/client/orgs"
	"github.com/grafana/grafana-openapi-client-go/client/sso_settings"
	"github.com/grafana/grafana-openapi-client-go/models"
	"github.com/stretchr/testify/mock"
)

// MockOrgsClient is a mock for the orgs client service
type MockOrgsClient struct {
	mock.Mock
}

func (m *MockOrgsClient) GetOrgByName(orgName string, opts ...orgs.ClientOption) (*orgs.GetOrgByNameOK, error) {
	args := m.Called(orgName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*orgs.GetOrgByNameOK), args.Error(1)
}

func (m *MockOrgsClient) GetOrgByNameWithParams(params *orgs.GetOrgByNameParams, opts ...orgs.ClientOption) (*orgs.GetOrgByNameOK, error) {
	return nil, nil
}

func (m *MockOrgsClient) CreateOrg(body *models.CreateOrgCommand, opts ...orgs.ClientOption) (*orgs.CreateOrgOK, error) {
	args := m.Called(body)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*orgs.CreateOrgOK), args.Error(1)
}

func (m *MockOrgsClient) CreateOrgWithParams(params *orgs.CreateOrgParams, opts ...orgs.ClientOption) (*orgs.CreateOrgOK, error) {
	return nil, nil
}

func (m *MockOrgsClient) UpdateOrg(orgID int64, body *models.UpdateOrgForm, opts ...orgs.ClientOption) (*orgs.UpdateOrgOK, error) {
	args := m.Called(orgID, body)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*orgs.UpdateOrgOK), args.Error(1)
}

func (m *MockOrgsClient) UpdateOrgWithParams(params *orgs.UpdateOrgParams, opts ...orgs.ClientOption) (*orgs.UpdateOrgOK, error) {
	return nil, nil
}

func (m *MockOrgsClient) DeleteOrgByID(orgID int64, opts ...orgs.ClientOption) (*orgs.DeleteOrgByIDOK, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*orgs.DeleteOrgByIDOK), args.Error(1)
}

func (m *MockOrgsClient) DeleteOrgByIDWithParams(params *orgs.DeleteOrgByIDParams, opts ...orgs.ClientOption) (*orgs.DeleteOrgByIDOK, error) {
	return nil, nil
}

func (m *MockOrgsClient) GetOrgByID(orgID int64, opts ...orgs.ClientOption) (*orgs.GetOrgByIDOK, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*orgs.GetOrgByIDOK), args.Error(1)
}

func (m *MockOrgsClient) GetOrgByIDWithParams(params *orgs.GetOrgByIDParams, opts ...orgs.ClientOption) (*orgs.GetOrgByIDOK, error) {
	return nil, nil
}

// For all other methods, use a generic approach that returns nil/error
func (m *MockOrgsClient) AddOrgUser(orgID int64, body *models.AddOrgUserCommand, opts ...orgs.ClientOption) (*orgs.AddOrgUserOK, error) {
	return nil, nil
}

func (m *MockOrgsClient) AddOrgUserWithParams(params *orgs.AddOrgUserParams, opts ...orgs.ClientOption) (*orgs.AddOrgUserOK, error) {
	return nil, nil
}

func (m *MockOrgsClient) GetOrgUsers(orgID int64, opts ...orgs.ClientOption) (*orgs.GetOrgUsersOK, error) {
	return nil, nil
}

func (m *MockOrgsClient) RemoveOrgUser(orgID int64, userID int64, opts ...orgs.ClientOption) (*orgs.RemoveOrgUserOK, error) {
	return nil, nil
}

func (m *MockOrgsClient) UpdateOrgUser(params *orgs.UpdateOrgUserParams, opts ...orgs.ClientOption) (*orgs.UpdateOrgUserOK, error) {
	return nil, nil
}

func (m *MockOrgsClient) UpdateOrgUserWithParams(params *orgs.UpdateOrgUserParams, opts ...orgs.ClientOption) (*orgs.UpdateOrgUserOK, error) {
	return nil, nil
}

func (m *MockOrgsClient) RemoveOrgUserWithParams(params *orgs.RemoveOrgUserParams, opts ...orgs.ClientOption) (*orgs.RemoveOrgUserOK, error) {
	return nil, nil
}

func (m *MockOrgsClient) GetOrgUsersWithParams(params *orgs.GetOrgUsersParams, opts ...orgs.ClientOption) (*orgs.GetOrgUsersOK, error) {
	return nil, nil
}

func (m *MockOrgsClient) GetOrgQuota(orgID int64, opts ...orgs.ClientOption) (*orgs.GetOrgQuotaOK, error) {
	return nil, nil
}

func (m *MockOrgsClient) GetOrgQuotaWithParams(params *orgs.GetOrgQuotaParams, opts ...orgs.ClientOption) (*orgs.GetOrgQuotaOK, error) {
	return nil, nil
}

func (m *MockOrgsClient) UpdateOrgQuota(params *orgs.UpdateOrgQuotaParams, opts ...orgs.ClientOption) (*orgs.UpdateOrgQuotaOK, error) {
	return nil, nil
}

func (m *MockOrgsClient) SearchOrgs(params *orgs.SearchOrgsParams, opts ...orgs.ClientOption) (*orgs.SearchOrgsOK, error) {
	return nil, nil
}

func (m *MockOrgsClient) SearchOrgsWithParams(params *orgs.SearchOrgsParams, opts ...orgs.ClientOption) (*orgs.SearchOrgsOK, error) {
	return nil, nil
}

func (m *MockOrgsClient) SearchOrgUsers(orgID int64, opts ...orgs.ClientOption) (*orgs.SearchOrgUsersOK, error) {
	return nil, nil
}

func (m *MockOrgsClient) SearchOrgUsersWithParams(params *orgs.SearchOrgUsersParams, opts ...orgs.ClientOption) (*orgs.SearchOrgUsersOK, error) {
	return nil, nil
}

func (m *MockOrgsClient) UpdateOrgAddress(orgID int64, body *models.UpdateOrgAddressForm, opts ...orgs.ClientOption) (*orgs.UpdateOrgAddressOK, error) {
	return nil, nil
}

func (m *MockOrgsClient) UpdateOrgAddressWithParams(params *orgs.UpdateOrgAddressParams, opts ...orgs.ClientOption) (*orgs.UpdateOrgAddressOK, error) {
	return nil, nil
}

func (m *MockOrgsClient) SetTransport(transport runtime.ClientTransport) {}

// MockDashboardsClient is a mock for the dashboards client service
type MockDashboardsClient struct {
	mock.Mock
}

func (m *MockDashboardsClient) PostDashboard(body *models.SaveDashboardCommand, opts ...dashboards.ClientOption) (*dashboards.PostDashboardOK, error) {
	args := m.Called(body)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dashboards.PostDashboardOK), args.Error(1)
}

func (m *MockDashboardsClient) PostDashboardWithParams(params *dashboards.PostDashboardParams, opts ...dashboards.ClientOption) (*dashboards.PostDashboardOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) GetDashboardByUID(uid string, opts ...dashboards.ClientOption) (*dashboards.GetDashboardByUIDOK, error) {
	args := m.Called(uid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dashboards.GetDashboardByUIDOK), args.Error(1)
}

func (m *MockDashboardsClient) GetDashboardByUIDWithParams(params *dashboards.GetDashboardByUIDParams, opts ...dashboards.ClientOption) (*dashboards.GetDashboardByUIDOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) DeleteDashboardByUID(uid string, opts ...dashboards.ClientOption) (*dashboards.DeleteDashboardByUIDOK, error) {
	args := m.Called(uid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dashboards.DeleteDashboardByUIDOK), args.Error(1)
}

func (m *MockDashboardsClient) DeleteDashboardByUIDWithParams(params *dashboards.DeleteDashboardByUIDParams, opts ...dashboards.ClientOption) (*dashboards.DeleteDashboardByUIDOK, error) {
	return nil, nil
}

// For all other methods, use a generic approach that returns nil/error
func (m *MockDashboardsClient) CalculateDashboardDiff(body *models.CalculateDashboardDiffParamsBody, opts ...dashboards.ClientOption) (*dashboards.CalculateDashboardDiffOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) CalculateDashboardDiffWithParams(params *dashboards.CalculateDashboardDiffParams, opts ...dashboards.ClientOption) (*dashboards.CalculateDashboardDiffOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) GetHomeDashboard(opts ...dashboards.ClientOption) (*dashboards.GetHomeDashboardOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) GetHomeDashboardWithParams(params *dashboards.GetHomeDashboardParams, opts ...dashboards.ClientOption) (*dashboards.GetHomeDashboardOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) SearchDashboards(opts ...dashboards.ClientOption) (interface{}, error) {
	return nil, nil
}

func (m *MockDashboardsClient) GetDashboardTags(opts ...dashboards.ClientOption) (*dashboards.GetDashboardTagsOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) GetDashboardTagsWithParams(params *dashboards.GetDashboardTagsParams, opts ...dashboards.ClientOption) (*dashboards.GetDashboardTagsOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) ImportDashboard(body *models.ImportDashboardRequest, opts ...dashboards.ClientOption) (*dashboards.ImportDashboardOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) ImportDashboardWithParams(params *dashboards.ImportDashboardParams, opts ...dashboards.ClientOption) (*dashboards.ImportDashboardOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) SetTransport(transport runtime.ClientTransport) {}

// MockDatasourcesClient is a mock for the datasources client service
type MockDatasourcesClient struct {
	mock.Mock
}

func (m *MockDatasourcesClient) AddDataSource(body *models.AddDataSourceCommand, opts ...datasources.ClientOption) (*datasources.AddDataSourceOK, error) {
	args := m.Called(body)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*datasources.AddDataSourceOK), args.Error(1)
}

func (m *MockDatasourcesClient) UpdateDataSourceByUID(uid string, body *models.UpdateDataSourceCommand, opts ...datasources.ClientOption) (*datasources.UpdateDataSourceByUIDOK, error) {
	args := m.Called(uid, body)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*datasources.UpdateDataSourceByUIDOK), args.Error(1)
}

func (m *MockDatasourcesClient) UpdateDataSourceByUIDWithParams(params *datasources.UpdateDataSourceByUIDParams, opts ...datasources.ClientOption) (*datasources.UpdateDataSourceByUIDOK, error) {
	return nil, nil
}

func (m *MockDatasourcesClient) DeleteDataSourceByUID(uid string, opts ...datasources.ClientOption) (*datasources.DeleteDataSourceByUIDOK, error) {
	args := m.Called(uid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*datasources.DeleteDataSourceByUIDOK), args.Error(1)
}

func (m *MockDatasourcesClient) DeleteDataSourceByUIDWithParams(params *datasources.DeleteDataSourceByUIDParams, opts ...datasources.ClientOption) (*datasources.DeleteDataSourceByUIDOK, error) {
	return nil, nil
}

func (m *MockDatasourcesClient) GetDataSources(opts ...datasources.ClientOption) (*datasources.GetDataSourcesOK, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*datasources.GetDataSourcesOK), args.Error(1)
}

func (m *MockDatasourcesClient) GetDataSourcesWithParams(params *datasources.GetDataSourcesParams, opts ...datasources.ClientOption) (*datasources.GetDataSourcesOK, error) {
	return nil, nil
}

// For all other methods, use a generic approach that returns nil/error
func (m *MockDatasourcesClient) AddDataSourceWithParams(params *datasources.AddDataSourceParams, opts ...datasources.ClientOption) (*datasources.AddDataSourceOK, error) {
	return nil, nil
}

func (m *MockDatasourcesClient) CheckDatasourceHealthByUID(uid string, opts ...datasources.ClientOption) (interface{}, error) {
	return nil, nil
}

func (m *MockDatasourcesClient) CheckDatasourceHealthByUIDWithParams(params interface{}, opts ...datasources.ClientOption) (interface{}, error) {
	return nil, nil
}

func (m *MockDatasourcesClient) CheckDatasourceHealthByID(id string, opts ...datasources.ClientOption) (interface{}, error) {
	return nil, nil
}

func (m *MockDatasourcesClient) DeleteDataSourceByName(name string, opts ...datasources.ClientOption) (*datasources.DeleteDataSourceByNameOK, error) {
	return nil, nil
}

func (m *MockDatasourcesClient) DeleteDataSourceByNameWithParams(params *datasources.DeleteDataSourceByNameParams, opts ...datasources.ClientOption) (*datasources.DeleteDataSourceByNameOK, error) {
	return nil, nil
}

func (m *MockDatasourcesClient) GetDataSourceByName(name string, opts ...datasources.ClientOption) (*datasources.GetDataSourceByNameOK, error) {
	return nil, nil
}

func (m *MockDatasourcesClient) GetDataSourceByNameWithParams(params *datasources.GetDataSourceByNameParams, opts ...datasources.ClientOption) (*datasources.GetDataSourceByNameOK, error) {
	return nil, nil
}

func (m *MockDatasourcesClient) GetDataSourceByUID(uid string, opts ...datasources.ClientOption) (*datasources.GetDataSourceByUIDOK, error) {
	return nil, nil
}

func (m *MockDatasourcesClient) GetDataSourceByUIDWithParams(params *datasources.GetDataSourceByUIDParams, opts ...datasources.ClientOption) (*datasources.GetDataSourceByUIDOK, error) {
	return nil, nil
}

func (m *MockDatasourcesClient) CallDatasourceResourceByID(id string, resourcePath string, opts ...datasources.ClientOption) (*datasources.CallDatasourceResourceByIDOK, error) {
	return nil, nil
}

func (m *MockDatasourcesClient) CallDatasourceResourceByIDWithParams(params *datasources.CallDatasourceResourceByIDParams, opts ...datasources.ClientOption) (*datasources.CallDatasourceResourceByIDOK, error) {
	return nil, nil
}

func (m *MockDatasourcesClient) CallDatasourceResourceWithUID(uid string, resourcePath string, opts ...datasources.ClientOption) (*datasources.CallDatasourceResourceWithUIDOK, error) {
	return nil, nil
}

func (m *MockDatasourcesClient) CallDatasourceResourceWithUIDWithParams(params *datasources.CallDatasourceResourceWithUIDParams, opts ...datasources.ClientOption) (*datasources.CallDatasourceResourceWithUIDOK, error) {
	return nil, nil
}

func (m *MockDatasourcesClient) SetTransport(transport runtime.ClientTransport) {}

// MockSsoSettingsClient is a mock for the sso_settings client service
type MockSsoSettingsClient struct {
	mock.Mock
}

func (m *MockSsoSettingsClient) GetProviderSettings(provider string, opts ...sso_settings.ClientOption) (*sso_settings.GetProviderSettingsOK, error) {
	args := m.Called(provider)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sso_settings.GetProviderSettingsOK), args.Error(1)
}

func (m *MockSsoSettingsClient) UpdateProviderSettings(provider string, body *models.UpdateProviderSettingsParamsBody, opts ...sso_settings.ClientOption) (*sso_settings.UpdateProviderSettingsNoContent, error) {
	args := m.Called(provider, body)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sso_settings.UpdateProviderSettingsNoContent), args.Error(1)
}

// For all other methods, use a generic approach that returns nil/error
func (m *MockSsoSettingsClient) DeleteProviderSettings(provider string, opts ...sso_settings.ClientOption) (interface{}, error) {
	return nil, nil
}

func (m *MockSsoSettingsClient) GetSSOSettings(opts ...sso_settings.ClientOption) (interface{}, error) {
	return nil, nil
}

func (m *MockSsoSettingsClient) GetProviderSettingsWithParams(params *sso_settings.GetProviderSettingsParams, opts ...sso_settings.ClientOption) (*sso_settings.GetProviderSettingsOK, error) {
	return nil, nil
}

func (m *MockSsoSettingsClient) ListAllProvidersSettings(opts ...sso_settings.ClientOption) (*sso_settings.ListAllProvidersSettingsOK, error) {
	return nil, nil
}

func (m *MockSsoSettingsClient) ListAllProvidersSettingsWithParams(params *sso_settings.ListAllProvidersSettingsParams, opts ...sso_settings.ClientOption) (*sso_settings.ListAllProvidersSettingsOK, error) {
	return nil, nil
}

func (m *MockSsoSettingsClient) RemoveProviderSettings(provider string, opts ...sso_settings.ClientOption) (*sso_settings.RemoveProviderSettingsNoContent, error) {
	return nil, nil
}

func (m *MockSsoSettingsClient) RemoveProviderSettingsWithParams(params *sso_settings.RemoveProviderSettingsParams, opts ...sso_settings.ClientOption) (*sso_settings.RemoveProviderSettingsNoContent, error) {
	return nil, nil
}

func (m *MockSsoSettingsClient) SetTransport(transport runtime.ClientTransport) {}
