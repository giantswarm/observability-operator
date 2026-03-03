package mocks

import (
	"github.com/go-openapi/runtime"
	"github.com/grafana/grafana-openapi-client-go/client/dashboards"
	"github.com/grafana/grafana-openapi-client-go/client/datasources"
	"github.com/grafana/grafana-openapi-client-go/client/folders"
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

// Note: GetOrgQuota and UpdateOrgQuota methods have been removed from the Grafana OpenAPI client

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
// Note: CalculateDashboardDiff methods have been removed from the Grafana OpenAPI client

func (m *MockDashboardsClient) CreateDashboardSnapshot(body *models.CreateDashboardSnapshotCommand, opts ...dashboards.ClientOption) (*dashboards.CreateDashboardSnapshotOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) CreateDashboardSnapshotWithParams(params *dashboards.CreateDashboardSnapshotParams, opts ...dashboards.ClientOption) (*dashboards.CreateDashboardSnapshotOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) CreatePublicDashboard(dashboardUID string, body *models.PublicDashboardDTO, opts ...dashboards.ClientOption) (*dashboards.CreatePublicDashboardOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) CreatePublicDashboardWithParams(params *dashboards.CreatePublicDashboardParams, opts ...dashboards.ClientOption) (*dashboards.CreatePublicDashboardOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) DeleteDashboardSnapshot(key string, opts ...dashboards.ClientOption) (*dashboards.DeleteDashboardSnapshotOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) DeleteDashboardSnapshotWithParams(params *dashboards.DeleteDashboardSnapshotParams, opts ...dashboards.ClientOption) (*dashboards.DeleteDashboardSnapshotOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) DeleteDashboardSnapshotByDeleteKey(deleteKey string, opts ...dashboards.ClientOption) (*dashboards.DeleteDashboardSnapshotByDeleteKeyOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) DeleteDashboardSnapshotByDeleteKeyWithParams(params *dashboards.DeleteDashboardSnapshotByDeleteKeyParams, opts ...dashboards.ClientOption) (*dashboards.DeleteDashboardSnapshotByDeleteKeyOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) DeletePublicDashboard(uid string, dashboardUID string, opts ...dashboards.ClientOption) (*dashboards.DeletePublicDashboardOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) DeletePublicDashboardWithParams(params *dashboards.DeletePublicDashboardParams, opts ...dashboards.ClientOption) (*dashboards.DeletePublicDashboardOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) GetDashboardPermissionsListByID(dashboardID int64, opts ...dashboards.ClientOption) (*dashboards.GetDashboardPermissionsListByIDOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) GetDashboardPermissionsListByIDWithParams(params *dashboards.GetDashboardPermissionsListByIDParams, opts ...dashboards.ClientOption) (*dashboards.GetDashboardPermissionsListByIDOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) GetDashboardPermissionsListByUID(uid string, opts ...dashboards.ClientOption) (*dashboards.GetDashboardPermissionsListByUIDOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) GetDashboardPermissionsListByUIDWithParams(params *dashboards.GetDashboardPermissionsListByUIDParams, opts ...dashboards.ClientOption) (*dashboards.GetDashboardPermissionsListByUIDOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) GetDashboardSnapshot(key string, opts ...dashboards.ClientOption) (*dashboards.GetDashboardSnapshotOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) GetDashboardSnapshotWithParams(params *dashboards.GetDashboardSnapshotParams, opts ...dashboards.ClientOption) (*dashboards.GetDashboardSnapshotOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) GetDashboardVersionByID(dashboardVersionID int64, dashboardID int64, opts ...dashboards.ClientOption) (*dashboards.GetDashboardVersionByIDOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) GetDashboardVersionByIDWithParams(params *dashboards.GetDashboardVersionByIDParams, opts ...dashboards.ClientOption) (*dashboards.GetDashboardVersionByIDOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) GetDashboardVersionByUID(uid string, dashboardVersionID int64, opts ...dashboards.ClientOption) (*dashboards.GetDashboardVersionByUIDOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) GetDashboardVersionByUIDWithParams(params *dashboards.GetDashboardVersionByUIDParams, opts ...dashboards.ClientOption) (*dashboards.GetDashboardVersionByUIDOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) GetDashboardVersionsByID(dashboardID int64, opts ...dashboards.ClientOption) (*dashboards.GetDashboardVersionsByIDOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) GetDashboardVersionsByIDWithParams(params *dashboards.GetDashboardVersionsByIDParams, opts ...dashboards.ClientOption) (*dashboards.GetDashboardVersionsByIDOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) GetDashboardVersionsByUID(params *dashboards.GetDashboardVersionsByUIDParams, opts ...dashboards.ClientOption) (*dashboards.GetDashboardVersionsByUIDOK, error) {
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

func (m *MockDashboardsClient) GetPublicAnnotations(accessToken string, opts ...dashboards.ClientOption) (*dashboards.GetPublicAnnotationsOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) GetPublicAnnotationsWithParams(params *dashboards.GetPublicAnnotationsParams, opts ...dashboards.ClientOption) (*dashboards.GetPublicAnnotationsOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) GetPublicDashboard(dashboardUID string, opts ...dashboards.ClientOption) (*dashboards.GetPublicDashboardOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) GetPublicDashboardWithParams(params *dashboards.GetPublicDashboardParams, opts ...dashboards.ClientOption) (*dashboards.GetPublicDashboardOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) ImportDashboard(body *models.ImportDashboardRequest, opts ...dashboards.ClientOption) (*dashboards.ImportDashboardOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) ImportDashboardWithParams(params *dashboards.ImportDashboardParams, opts ...dashboards.ClientOption) (*dashboards.ImportDashboardOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) InterpolateDashboard(opts ...dashboards.ClientOption) (*dashboards.InterpolateDashboardOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) InterpolateDashboardWithParams(params *dashboards.InterpolateDashboardParams, opts ...dashboards.ClientOption) (*dashboards.InterpolateDashboardOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) ListPublicDashboards(opts ...dashboards.ClientOption) (*dashboards.ListPublicDashboardsOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) ListPublicDashboardsWithParams(params *dashboards.ListPublicDashboardsParams, opts ...dashboards.ClientOption) (*dashboards.ListPublicDashboardsOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) QueryPublicDashboard(panelID int64, accessToken string, opts ...dashboards.ClientOption) (*dashboards.QueryPublicDashboardOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) QueryPublicDashboardWithParams(params *dashboards.QueryPublicDashboardParams, opts ...dashboards.ClientOption) (*dashboards.QueryPublicDashboardOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) RestoreDashboardVersionByUID(uid string, body *models.RestoreDashboardVersionCommand, opts ...dashboards.ClientOption) (*dashboards.RestoreDashboardVersionByUIDOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) RestoreDashboardVersionByUIDWithParams(params *dashboards.RestoreDashboardVersionByUIDParams, opts ...dashboards.ClientOption) (*dashboards.RestoreDashboardVersionByUIDOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) SearchDashboardSnapshots(params *dashboards.SearchDashboardSnapshotsParams, opts ...dashboards.ClientOption) (*dashboards.SearchDashboardSnapshotsOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) UpdateDashboardPermissionsByID(dashboardID int64, body *models.UpdateDashboardACLCommand, opts ...dashboards.ClientOption) (*dashboards.UpdateDashboardPermissionsByIDOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) UpdateDashboardPermissionsByIDWithParams(params *dashboards.UpdateDashboardPermissionsByIDParams, opts ...dashboards.ClientOption) (*dashboards.UpdateDashboardPermissionsByIDOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) UpdateDashboardPermissionsByUID(uid string, body *models.UpdateDashboardACLCommand, opts ...dashboards.ClientOption) (*dashboards.UpdateDashboardPermissionsByUIDOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) UpdateDashboardPermissionsByUIDWithParams(params *dashboards.UpdateDashboardPermissionsByUIDParams, opts ...dashboards.ClientOption) (*dashboards.UpdateDashboardPermissionsByUIDOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) UpdatePublicDashboard(params *dashboards.UpdatePublicDashboardParams, opts ...dashboards.ClientOption) (*dashboards.UpdatePublicDashboardOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) ViewPublicDashboard(accessToken string, opts ...dashboards.ClientOption) (*dashboards.ViewPublicDashboardOK, error) {
	return nil, nil
}

func (m *MockDashboardsClient) ViewPublicDashboardWithParams(params *dashboards.ViewPublicDashboardParams, opts ...dashboards.ClientOption) (*dashboards.ViewPublicDashboardOK, error) {
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

// MockFoldersClient is a mock for the folders client service
type MockFoldersClient struct {
	mock.Mock
}

func (m *MockFoldersClient) CreateFolder(body *models.CreateFolderCommand, opts ...folders.ClientOption) (*folders.CreateFolderOK, error) {
	args := m.Called(body)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*folders.CreateFolderOK), args.Error(1)
}

func (m *MockFoldersClient) CreateFolderWithParams(params *folders.CreateFolderParams, opts ...folders.ClientOption) (*folders.CreateFolderOK, error) {
	return nil, nil
}

func (m *MockFoldersClient) DeleteFolder(params *folders.DeleteFolderParams, opts ...folders.ClientOption) (*folders.DeleteFolderOK, error) {
	args := m.Called(params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*folders.DeleteFolderOK), args.Error(1)
}

func (m *MockFoldersClient) GetFolderByID(folderID int64, opts ...folders.ClientOption) (*folders.GetFolderByIDOK, error) {
	return nil, nil
}

func (m *MockFoldersClient) GetFolderByIDWithParams(params *folders.GetFolderByIDParams, opts ...folders.ClientOption) (*folders.GetFolderByIDOK, error) {
	return nil, nil
}

func (m *MockFoldersClient) GetFolderByUID(folderUID string, opts ...folders.ClientOption) (*folders.GetFolderByUIDOK, error) {
	args := m.Called(folderUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*folders.GetFolderByUIDOK), args.Error(1)
}

func (m *MockFoldersClient) GetFolderByUIDWithParams(params *folders.GetFolderByUIDParams, opts ...folders.ClientOption) (*folders.GetFolderByUIDOK, error) {
	return nil, nil
}

func (m *MockFoldersClient) GetFolderDescendantCounts(folderUID string, opts ...folders.ClientOption) (*folders.GetFolderDescendantCountsOK, error) {
	args := m.Called(folderUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*folders.GetFolderDescendantCountsOK), args.Error(1)
}

func (m *MockFoldersClient) GetFolderDescendantCountsWithParams(params *folders.GetFolderDescendantCountsParams, opts ...folders.ClientOption) (*folders.GetFolderDescendantCountsOK, error) {
	return nil, nil
}

func (m *MockFoldersClient) GetFolderPermissionList(folderUID string, opts ...folders.ClientOption) (*folders.GetFolderPermissionListOK, error) {
	return nil, nil
}

func (m *MockFoldersClient) GetFolderPermissionListWithParams(params *folders.GetFolderPermissionListParams, opts ...folders.ClientOption) (*folders.GetFolderPermissionListOK, error) {
	return nil, nil
}

func (m *MockFoldersClient) GetFolders(params *folders.GetFoldersParams, opts ...folders.ClientOption) (*folders.GetFoldersOK, error) {
	args := m.Called(params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*folders.GetFoldersOK), args.Error(1)
}

func (m *MockFoldersClient) MoveFolder(folderUID string, body *models.MoveFolderCommand, opts ...folders.ClientOption) (*folders.MoveFolderOK, error) {
	return nil, nil
}

func (m *MockFoldersClient) MoveFolderWithParams(params *folders.MoveFolderParams, opts ...folders.ClientOption) (*folders.MoveFolderOK, error) {
	return nil, nil
}

func (m *MockFoldersClient) UpdateFolder(folderUID string, body *models.UpdateFolderCommand, opts ...folders.ClientOption) (*folders.UpdateFolderOK, error) {
	args := m.Called(folderUID, body)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*folders.UpdateFolderOK), args.Error(1)
}

func (m *MockFoldersClient) UpdateFolderWithParams(params *folders.UpdateFolderParams, opts ...folders.ClientOption) (*folders.UpdateFolderOK, error) {
	return nil, nil
}

func (m *MockFoldersClient) UpdateFolderPermissions(folderUID string, body *models.UpdateDashboardACLCommand, opts ...folders.ClientOption) (*folders.UpdateFolderPermissionsOK, error) {
	return nil, nil
}

func (m *MockFoldersClient) UpdateFolderPermissionsWithParams(params *folders.UpdateFolderPermissionsParams, opts ...folders.ClientOption) (*folders.UpdateFolderPermissionsOK, error) {
	return nil, nil
}

func (m *MockFoldersClient) SetTransport(transport runtime.ClientTransport) {}
