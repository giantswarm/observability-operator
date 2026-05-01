package client

import (
	"github.com/grafana/grafana-openapi-client-go/client/dashboards"
	"github.com/grafana/grafana-openapi-client-go/client/datasources"
	"github.com/grafana/grafana-openapi-client-go/client/folders"
	"github.com/grafana/grafana-openapi-client-go/client/orgs"
	"github.com/grafana/grafana-openapi-client-go/client/sso_settings"
)

// GrafanaClient exposes the Grafana HTTP API operations the operator uses.
//
// WithOrgID returns a new client scoped to the given organization. The receiver
// is left untouched, so the same base client may be reused safely from
// concurrent goroutines.
type GrafanaClient interface {
	WithOrgID(orgID int64) GrafanaClient

	Datasources() datasources.ClientService
	Orgs() orgs.ClientService
	Dashboards() dashboards.ClientService
	Folders() folders.ClientService
	SsoSettings() sso_settings.ClientService
}
