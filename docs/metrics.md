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
```

## Adding new metrics

Metrics are defined in `pkg/metrics/metrics.go` and registered via `init()`. See [CONTRIBUTING.md](../CONTRIBUTING.md#metrics) for the pattern.
