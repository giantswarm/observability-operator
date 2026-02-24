package client

import (
	"github.com/grafana/grafana-openapi-client-go/client/dashboards"
	"github.com/grafana/grafana-openapi-client-go/client/datasources"
	"github.com/grafana/grafana-openapi-client-go/client/folders"
	"github.com/grafana/grafana-openapi-client-go/client/orgs"
	"github.com/grafana/grafana-openapi-client-go/client/sso_settings"
)

// GrafanaClient defines the unified interface that provides access to all Grafana API operations
// This interface allows for easier testing by providing a single point
// to mock all Grafana API operations through focused client services.
type GrafanaClient interface {
	// Organization context management
	OrgID() int64
	WithOrgID(orgID int64) GrafanaClient

	// Client services - expose focused client interfaces
	Datasources() datasources.ClientService
	Orgs() orgs.ClientService
	Dashboards() dashboards.ClientService
	Folders() folders.ClientService
	SsoSettings() sso_settings.ClientService
}
