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
	GrafanaOrganizationTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "observability_operator_grafana_organizations_total",
		Help: "Total number of GrafanaOrganization resources",
	}, []string{"status"}) // status: active, pending, error

	GrafanaOrganizationReconciliations = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "observability_operator_grafana_organization_reconciliations_total",
		Help: "Total number of GrafanaOrganization reconciliations",
	}, []string{"name", "result"}) // result: success, error

	GrafanaOrganizationReconciliationDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "observability_operator_grafana_organization_reconciliation_duration_seconds",
		Help:    "Time taken to reconcile GrafanaOrganization resources",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 15), // 1ms to ~30s
	}, []string{"name"})

	GrafanaOrganizationOperations = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "observability_operator_grafana_organization_operations_total",
		Help: "Total number of GrafanaOrganization operations",
	}, []string{"name", "operation", "result"}) // operation: create, update, delete, setup_datasources, configure_sso

	GrafanaOrganizationDataSources = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "observability_operator_grafana_organization_datasources",
		Help: "Number of data sources configured for each GrafanaOrganization",
	}, []string{"name", "org_id"})

	GrafanaOrganizationTenants = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "observability_operator_grafana_organization_tenants",
		Help: "Number of tenants associated with each GrafanaOrganization",
	}, []string{"name", "org_id"})

	GrafanaOrganizationAge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "observability_operator_grafana_organization_age_seconds",
		Help: "Age of GrafanaOrganization resources in seconds",
	}, []string{"name", "org_id"})

	GrafanaOrganizationInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "observability_operator_grafana_organization_info",
		Help: "Information about GrafanaOrganization resources",
	}, []string{"name", "display_name", "org_id", "has_finalizer"})

	GrafanaOrganizationErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "observability_operator_grafana_organization_errors_total",
		Help: "Total number of errors encountered while managing GrafanaOrganization resources",
	}, []string{"name", "error_type"}) // error_type: grafana_api, kubernetes_api, configuration, validation
)

func init() {
	metrics.Registry.MustRegister(
		MimirQueryErrors,
		GrafanaOrganizationTotal,
		GrafanaOrganizationReconciliations,
		GrafanaOrganizationReconciliationDuration,
		GrafanaOrganizationOperations,
		GrafanaOrganizationDataSources,
		GrafanaOrganizationTenants,
		GrafanaOrganizationAge,
		GrafanaOrganizationInfo,
		GrafanaOrganizationErrors,
	)
}
