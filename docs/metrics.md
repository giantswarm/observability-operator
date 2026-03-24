# Operator Metrics

The operator exposes Prometheus-compatible metrics via the standard controller-runtime endpoint, scraped by the PodMonitor at `helm/observability-operator/templates/pod-monitor.yaml`.

In addition to the [standard controller-runtime metrics](https://book.kubebuilder.io/reference/metrics-reference), the operator exposes the following custom metrics (all prefixed `observability_operator_`).

## `observability_operator_grafana_organization_info`

Info gauge tracking each `GrafanaOrganization`. Always `1`.

| Label | Description |
|---|---|
| `name` | GrafanaOrganization resource name |
| `display_name` | Display name shown in Grafana |
| `org_id` | Grafana org ID (`0` if not yet created in Grafana) |
| `status` | `active`, `pending`, or `error` |

## `observability_operator_grafana_organization_tenants`

Info gauge — one series per (tenant, org) pair. Always `1`.

| Label | Description |
|---|---|
| `name` | Tenant name |
| `org_id` | Grafana org ID |

To get tenant count per org: `count(observability_operator_grafana_organization_tenants) by (org_id)`

## `observability_operator_alertmanager_routes`

Gauge tracking the number of Alertmanager routes configured per tenant.

| Label | Description |
|---|---|
| `tenant` | Tenant name |

## `observability_operator_mimir_head_series_query_errors_total`

Counter incremented each time the operator fails to query Mimir for head series count (used by the sharding autoscaler).

No labels.

## `observability_operator_monitored_cluster_info`

Info gauge with one series per cluster currently being monitored. Always `1`.

| Label | Description |
|---|---|
| `cluster_name` | Cluster name |
| `cluster_namespace` | Cluster namespace |

Populated on every successful reconcile; series removed when a cluster is deleted.

## `observability_operator_grafana_api_errors_total`

Counter incremented each time the operator receives an error from the Grafana API.

| Label | Description |
|---|---|
| `operation` | `configure_org`, `delete_org`, `configure_datasources`, `configure_dashboard`, `delete_dashboard` |

## `observability_operator_mimir_alertmanager_api_errors_total`

Counter incremented each time the operator receives an error from the Mimir Alertmanager API.

| Label | Description |
|---|---|
| `operation` | `push_config`, `delete_config` |

## `observability_operator_ruler_api_errors_total`

Counter incremented each time the operator receives an error from the ruler API (Mimir or Loki).

| Label | Description |
|---|---|
| `operation` | `delete_rules` |

## Example queries

```promql
# Organizations not yet provisioned in Grafana
observability_operator_grafana_organization_info{org_id="0"}

# Count of organizations by status
count(observability_operator_grafana_organization_info) by (status)

# Organizations in a non-active state (need attention)
observability_operator_grafana_organization_info{status!="active"}

# Tenant count per org
count(observability_operator_grafana_organization_tenants) by (org_id)

# Top 5 orgs by tenant count
topk(5, count(observability_operator_grafana_organization_tenants) by (org_id))

# Alertmanager route count per tenant
observability_operator_alertmanager_routes

# Total number of clusters being monitored
count(observability_operator_monitored_cluster_info)

# Grafana API error rate (per operation, 5m window)
rate(observability_operator_grafana_api_errors_total[5m])

# Mimir Alertmanager API error rate
rate(observability_operator_mimir_alertmanager_api_errors_total[5m])

# Any ruler deletion errors in the last hour
increase(observability_operator_ruler_api_errors_total[1h])
```

## Adding new metrics

Metrics are defined in `pkg/metrics/metrics.go` and registered via `init()`. See [CONTRIBUTING.md](../CONTRIBUTING.md#metrics) for the pattern.
