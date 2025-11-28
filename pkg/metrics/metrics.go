package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	MimirQueryErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "observability_operator_mimir_head_series_query_errors_total",
		Help: "Total number of reconciliations error",
	}, nil)

	// GrafanaOrganization metrics
	GrafanaOrganizationTenantInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "observability_operator_grafana_organization_tenants",
		Help: "Information about tenant resources per organization",
	}, []string{"name", "org_id"})

	GrafanaOrganizationInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "observability_operator_grafana_organization_info",
		Help: "Information about GrafanaOrganization resources",
	}, []string{"name", "display_name", "org_id", "status"}) // status: active, pending, error

	// Alertmanager metrics
	AlertmanagerRoutes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "observability_operator_alertmanager_routes",
		Help: "Number of routes configured in Alertmanager per tenant",
	}, []string{"tenant"})
)

const (
	OrgStatusActive  = "active"
	OrgStatusPending = "pending"
	OrgStatusError   = "error"
)

func init() {
	metrics.Registry.MustRegister(
		MimirQueryErrors,
		GrafanaOrganizationTenantInfo,
		GrafanaOrganizationInfo,
		AlertmanagerRoutes,
	)
}
