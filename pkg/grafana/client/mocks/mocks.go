package mocks

import (
	"context"
	"net/url"

	"github.com/grafana/grafana-openapi-client-go/client/dashboards"
	"github.com/grafana/grafana-openapi-client-go/client/datasources"
	"github.com/grafana/grafana-openapi-client-go/client/orgs"
	"github.com/grafana/grafana-openapi-client-go/client/sso_settings"
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
	if args.Get(0) == nil {
		return m // Return self if mock returns nil
	}
	return args.Get(0).(grafanaclient.GrafanaClient)
}

func (m *MockGrafanaClient) Datasources() datasources.ClientService {
	args := m.Called()
	return args.Get(0).(datasources.ClientService)
}

func (m *MockGrafanaClient) Orgs() orgs.ClientService {
	args := m.Called()
	return args.Get(0).(orgs.ClientService)
}

func (m *MockGrafanaClient) Dashboards() dashboards.ClientService {
	args := m.Called()
	return args.Get(0).(dashboards.ClientService)
}

func (m *MockGrafanaClient) SsoSettings() sso_settings.ClientService {
	args := m.Called()
	return args.Get(0).(sso_settings.ClientService)
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
