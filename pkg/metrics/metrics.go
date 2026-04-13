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

	// External API error counters

	// GrafanaAPIErrors counts errors returned by the Grafana API, labelled by operation.
	// operation values: configure_org, delete_org, configure_datasources, configure_dashboard, delete_dashboard
	GrafanaAPIErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "observability_operator_grafana_api_errors_total",
		Help: "Total number of errors from the Grafana API, by operation",
	}, []string{"operation"})

	// MimirAlertmanagerAPIErrors counts errors returned by the Mimir Alertmanager API, labelled by operation.
	// operation values: push_config, delete_config
	MimirAlertmanagerAPIErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "observability_operator_mimir_alertmanager_api_errors_total",
		Help: "Total number of errors from the Mimir Alertmanager API, by operation",
	}, []string{"operation"})

	// RulerAPIErrors counts errors returned by the ruler API (Mimir or Loki), labelled by operation.
	// operation values: delete_rules
	RulerAPIErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "observability_operator_ruler_api_errors_total",
		Help: "Total number of errors from the ruler API, by operation",
	}, []string{"operation"})

	// Cluster monitoring metrics

	// MonitoredClusterInfo is an info gauge with one series per cluster currently being monitored.
	// Use count(observability_operator_monitored_cluster_info) to get the total cluster count.
	MonitoredClusterInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "observability_operator_monitored_cluster_info",
		Help: "Info gauge for clusters being monitored; one series per active cluster",
	}, []string{"cluster_name", "cluster_namespace"})
)

const (
	OrgStatusActive  = "active"
	OrgStatusPending = "pending"
	OrgStatusError   = "error"
)

// Operation label values for GrafanaAPIErrors.
const (
	OpConfigureOrg         = "configure_org"
	OpDeleteOrg            = "delete_org"
	OpConfigureDatasources = "configure_datasources"
	OpConfigureDashboard   = "configure_dashboard"
	OpDeleteDashboard      = "delete_dashboard"
)

// Operation label values for MimirAlertmanagerAPIErrors.
const (
	OpPushConfig   = "push_config"
	OpDeleteConfig = "delete_config"
)

// Operation label values for RulerAPIErrors.
const (
	OpDeleteRules = "delete_rules"
)

func init() {
	metrics.Registry.MustRegister(
		MimirQueryErrors,
		GrafanaOrganizationTenantInfo,
		GrafanaOrganizationInfo,
		AlertmanagerRoutes,
		GrafanaAPIErrors,
		MimirAlertmanagerAPIErrors,
		RulerAPIErrors,
		MonitoredClusterInfo,
	)
}
