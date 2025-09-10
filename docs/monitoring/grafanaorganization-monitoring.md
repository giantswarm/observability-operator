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

### Configuration Metrics

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
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
   - Should be <5% under normal conditions

### Custom Queries

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
