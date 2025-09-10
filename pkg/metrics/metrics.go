package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (

	// GrafanaOrganization metrics
	GrafanaOrganizationTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "observability_operator_grafana_organizations_total",
		Help: "Total number of GrafanaOrganization resources",
	}, []string{"status"}) // status: active, pending, error

	GrafanaOrganizationTenants = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "observability_operator_grafana_organization_tenants",
		Help: "Number of tenants associated with each GrafanaOrganization",
	}, []string{"name", "org_id"})

	GrafanaOrganizationInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "observability_operator_grafana_organization_info",
		Help: "Information about GrafanaOrganization resources",
	}, []string{"name", "display_name", "org_id", "has_finalizer"})
)

func init() {
	metrics.Registry.MustRegister(
		GrafanaOrganizationTotal,
		GrafanaOrganizationTenants,
		GrafanaOrganizationInfo,
	)
}
