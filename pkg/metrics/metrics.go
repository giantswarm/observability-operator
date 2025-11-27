package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// MimirQueryErrors is a counter for tracking the number of errors while trying to query Mimir.
	MimirQueryErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "observability_operator_mimir_head_series_query_errors_total",
		Help: "Total number of reconciliations error",
	}, nil)

	// GrafanaOrganizationTenantInfo is a gauge for tracking tenant resources per organization.
	GrafanaOrganizationTenantInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "observability_operator_grafana_organization_tenants",
		Help: "Information about tenant resources per organization",
	}, []string{"name", "org_id"})

	// GrafanaOrganizationInfo is a gauge for tracking information about GrafanaOrganization resources.
	GrafanaOrganizationInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "observability_operator_grafana_organization_info",
		Help: "Information about GrafanaOrganization resources",
	}, []string{"name", "display_name", "org_id", "status"}) // status: active, pending, error

	// ObservabilityPrometheusTargetInfo is a gauge for tracking information about Alloy Prometheus targets.
	ObservabilityPrometheusTargetInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "observability_operator_prometheus_target_info",
		Help: "Information about Alloy Prometheus targets",
	}, []string{"app", "scrape_job"})
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
		ObservabilityPrometheusTargetInfo,
	)
}
