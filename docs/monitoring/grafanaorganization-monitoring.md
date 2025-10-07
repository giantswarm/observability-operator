# GrafanaOrganization Monitoring Guide

This guide explains how to monitor GrafanaOrganization custom resources deployed in your Kubernetes cluster using the observability-operator's built-in metrics and monitoring capabilities.

## Overview

The observability-operator exposes prometheus-compatible metrics.

On top of the common metrics [common metrics exposed by the controller runtime](https://book.kubebuilder.io/reference/metrics-reference), the `grafanaOrganization` reconciler provides metrics per:

- organization
- tenant

## Available Metrics

The operator exposes the following metrics for GrafanaOrganization resources:

### Resource State Metric

| Metric | Type | Description | Labels | Values |
|--------|------|-------------|--------|--------|
| `observability_operator_grafana_organization_info` | Gauge | Information about GrafanaOrganization resources | `name`, `status` (active, pending, error),`display_name`, `org_id` | Always set to 1 |

### Configuration Metric

| Metric | Type | Description | Labels | Values |
|--------|------|-------------|--------|--------|
| `observability_operator_grafana_organization_tenant_info` | Gauge | Information about tenant resources per organization | `name`, `org_id` | Always set to 1 |

## Extending the code to provide more observability data

### Adding Custom Metrics

To add custom metrics:

1. Define the metric in `pkg/metrics/metrics.go`
2. Add metric recording in the adequate controller

Example:
```go
// In metrics.go
CustomMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
    Name: "observability_operator_grafana_organization_custom_metric",
    Help: "Custom metric for GrafanaOrganization",
}, []string{"name"})

// In controller
metrics.RecordCustomMetric(grafanaOrganization.Name, value)
```

See prometheus go package [upstream documentation](https://pkg.go.dev/github.com/prometheus/client_golang@v1.23.2/prometheus) for more information.
