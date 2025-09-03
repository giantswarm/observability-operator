# GrafanaOrganization Monitoring Guide

This guide explains how to monitor GrafanaOrganization custom resources deployed in your Kubernetes cluster using the observability-operator's built-in metrics and monitoring capabilities.

## Overview

The observability-operator provides comprehensive monitoring for GrafanaOrganization resources through:

- **Custom Prometheus metrics** - Detailed metrics about resource states, operations, and performance
- **Prometheus alerting rules** - Pre-configured alerts for common issues and failures
- **Grafana dashboard** - Visual monitoring dashboard for operations teams
- **Background metrics collection** - Automatic periodic collection of resource metrics

## Available Metrics

The operator exposes the following metrics for GrafanaOrganization resources:

### Resource State Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `observability_operator_grafana_organizations_total` | Gauge | Total number of GrafanaOrganization resources by status | `status` (active, pending, error) |
| `observability_operator_grafana_organization_info` | Gauge | Information about GrafanaOrganization resources | `name`, `display_name`, `org_id`, `has_finalizer` |
| `observability_operator_grafana_organization_age_seconds` | Gauge | Age of GrafanaOrganization resources in seconds | `name`, `org_id` |

### Operation Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `observability_operator_grafana_organization_reconciliations_total` | Counter | Total number of reconciliations | `name`, `result` (success, error) |
| `observability_operator_grafana_organization_reconciliation_duration_seconds` | Histogram | Time taken to reconcile resources | `name` |
| `observability_operator_grafana_organization_operations_total` | Counter | Total number of operations by type | `name`, `operation`, `result` |
| `observability_operator_grafana_organization_errors_total` | Counter | Total number of errors by type | `name`, `error_type` |

### Configuration Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `observability_operator_grafana_organization_datasources` | Gauge | Number of data sources per organization | `name`, `org_id` |
| `observability_operator_grafana_organization_tenants` | Gauge | Number of tenants per organization | `name`, `org_id` |

## Monitoring Best Practices

### Key Metrics to Watch

1. **Resource Status Distribution**
   ```promql
   observability_operator_grafana_organizations_total
   ```
   - Monitor for resources stuck in pending or error states

2. **Reconciliation Success Rate**
   ```promql
   grafanaorganization:reconciliation_success_rate
   ```
   - Should be >95% under normal conditions

3. **Operation Failure Rate**
   ```promql
   rate(observability_operator_grafana_organization_operations_total{result="error"}[5m]) / 
   rate(observability_operator_grafana_organization_operations_total[5m])
   ```
   - Should be <5% under normal conditions

4. **Reconciliation Duration**
   ```promql
   histogram_quantile(0.95, rate(observability_operator_grafana_organization_reconciliation_duration_seconds_bucket[5m]))
   ```
   - P95 should be <10s under normal conditions

### Custom Queries

#### Organizations by Age
```promql
observability_operator_grafana_organization_age_seconds / 86400
```

#### Data Source Coverage
```promql
observability_operator_grafana_organization_datasources > 0
```

#### Tenant Distribution
```promql
topk(10, observability_operator_grafana_organization_tenants)
```

## Advanced Configuration

### Adjusting Metrics Collection Frequency

The background metrics collector runs every 30 seconds by default. To adjust this:

1. Modify the interval in `cmd/main.go`:
   ```go
   backgroundCollector := metrics.NewBackgroundMetricsCollector(mgr.GetClient(), 60*time.Second) // 60 seconds
   ```

2. Rebuild and redeploy the operator

### Custom Alert Thresholds

To adjust alert thresholds, edit `config/prometheus/grafanaorganization-alerts.yaml`:

```yaml
# Example: Adjust reconciliation slow threshold from 30s to 60s
- alert: GrafanaOrganizationReconciliationSlow
  expr: histogram_quantile(0.95, rate(observability_operator_grafana_organization_reconciliation_duration_seconds_bucket[5m])) > 60
```

### Adding Custom Metrics

To add custom metrics:

1. Define the metric in `pkg/metrics/metrics.go`
2. Update the collector in `pkg/metrics/grafanaorganization_collector.go`
3. Add metric recording in the controller

Example:
```go
// In metrics.go
CustomMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
    Name: "observability_operator_grafana_organization_custom_metric",
    Help: "Custom metric for GrafanaOrganization",
}, []string{"name"})

// In controller
r.metricsCollector.RecordCustomMetric(grafanaOrganization.Name, value)
```
