# GrafanaOrganization Monitoring Queries

This document provides example PromQL queries for monitoring GrafanaOrganization resources.

## Basic Resource Monitoring

### Total Organizations by Status
```promql
# Get count of organizations by status
observability_operator_grafana_organizations_total

# Organizations that are active
observability_operator_grafana_organizations_total{status="active"}

# Organizations stuck in pending state
observability_operator_grafana_organizations_total{status="pending"}
```

### Organization Information
```promql
# Get all organization details
observability_operator_grafana_organization_info

# Organizations with finalizers
observability_operator_grafana_organization_info{has_finalizer="true"}

# Organizations without Grafana org ID (not yet created)
observability_operator_grafana_organization_info{org_id="0"}
```

## Performance Monitoring

### Reconciliation Success Rate
```promql
# Overall success rate across all organizations
sum(rate(observability_operator_grafana_organization_reconciliations_total{result="success"}[5m])) / 
sum(rate(observability_operator_grafana_organization_reconciliations_total[5m]))

# Success rate per organization
rate(observability_operator_grafana_organization_reconciliations_total{result="success"}[5m]) / 
rate(observability_operator_grafana_organization_reconciliations_total[5m])
```

### Reconciliation Duration
```promql
# Average reconciliation duration
rate(observability_operator_grafana_organization_reconciliation_duration_seconds_sum[5m]) /
rate(observability_operator_grafana_organization_reconciliation_duration_seconds_count[5m])

# 95th percentile reconciliation duration
histogram_quantile(0.95, rate(observability_operator_grafana_organization_reconciliation_duration_seconds_bucket[5m]))

# Organizations with slow reconciliation (>10s average)
(
  rate(observability_operator_grafana_organization_reconciliation_duration_seconds_sum[5m]) /
  rate(observability_operator_grafana_organization_reconciliation_duration_seconds_count[5m])
) > 10
```

### Operation Rates
```promql
# Rate of all operations
rate(observability_operator_grafana_organization_operations_total[5m])

# Rate of failed operations
rate(observability_operator_grafana_organization_operations_total{result="error"}[5m])

# Operations by type
sum(rate(observability_operator_grafana_organization_operations_total[5m])) by (operation)
```

## Error Monitoring

### Error Rates
```promql
# Total error rate
rate(observability_operator_grafana_organization_errors_total[5m])

# Errors by type
sum(rate(observability_operator_grafana_organization_errors_total[5m])) by (error_type)

# Organizations with high error rates (>1 error per minute)
sum(rate(observability_operator_grafana_organization_errors_total[5m])) by (name) > (1/60)
```

### Failure Analysis
```promql
# Operation failure rate
rate(observability_operator_grafana_organization_operations_total{result="error"}[5m]) /
rate(observability_operator_grafana_organization_operations_total[5m])

# Organizations with >10% operation failure rate
(
  rate(observability_operator_grafana_organization_operations_total{result="error"}[5m]) /
  rate(observability_operator_grafana_organization_operations_total[5m])
) > 0.1
```

## Configuration Monitoring

### Data Sources
```promql
# Organizations without data sources
observability_operator_grafana_organization_datasources == 0

# Average data sources per organization
avg(observability_operator_grafana_organization_datasources)

# Organizations with most data sources
topk(5, observability_operator_grafana_organization_datasources)
```

### Tenants
```promql
# Total tenants across all organizations
sum(observability_operator_grafana_organization_tenants)

# Organizations with most tenants
topk(5, observability_operator_grafana_organization_tenants)

# Organizations with single tenant
observability_operator_grafana_organization_tenants == 1
```

## Age and Lifecycle Monitoring

### Organization Age
```promql
# Organizations older than 30 days
observability_operator_grafana_organization_age_seconds > (30 * 24 * 3600)

# Average age of organizations in days
avg(observability_operator_grafana_organization_age_seconds) / 86400

# Oldest organizations
topk(5, observability_operator_grafana_organization_age_seconds)
```

## Health Checks

### Overall Health
```promql
# Percentage of healthy organizations (active status)
(
  observability_operator_grafana_organizations_total{status="active"} /
  sum(observability_operator_grafana_organizations_total)
) * 100

# Organizations that should be investigated
observability_operator_grafana_organizations_total{status!="active"}
```

### Data Completeness
```promql
# Organizations with complete setup (has org_id and data sources)
observability_operator_grafana_organization_info{org_id!="0"} 
and on(name) 
observability_operator_grafana_organization_datasources > 0

# Organizations missing data sources but have org_id
observability_operator_grafana_organization_info{org_id!="0"} 
unless on(name) 
observability_operator_grafana_organization_datasources > 0
```

## Alerting Queries

### Critical Issues
```promql
# Organizations with reconciliation failures in last 5 minutes
increase(observability_operator_grafana_organization_reconciliations_total{result="error"}[5m]) > 0

# High error rate organizations
increase(observability_operator_grafana_organization_errors_total[10m]) > 10
```

### Performance Issues
```promql
# Slow reconciliation (95th percentile > 30s)
histogram_quantile(0.95, rate(observability_operator_grafana_organization_reconciliation_duration_seconds_bucket[5m])) > 30

# High operation failure rate
(
  rate(observability_operator_grafana_organization_operations_total{result="error"}[5m]) / 
  rate(observability_operator_grafana_organization_operations_total[5m])
) > 0.1
```

## Dashboard Queries

### Summary Stats for Dashboard
```promql
# For stat panels
sum(observability_operator_grafana_organizations_total)
observability_operator_grafana_organizations_total{status="active"}
observability_operator_grafana_organizations_total{status="pending"}
observability_operator_grafana_organizations_total{status="error"}

# For time series
rate(observability_operator_grafana_organization_reconciliations_total[5m])
histogram_quantile(0.95, rate(observability_operator_grafana_organization_reconciliation_duration_seconds_bucket[5m]))

# For tables
observability_operator_grafana_organization_info
observability_operator_grafana_organization_datasources
```

## Custom Recording Rules

You can create custom recording rules for commonly used queries:

```yaml
# Example recording rules
- record: grafanaorg:total_by_status
  expr: sum(observability_operator_grafana_organizations_total) by (status)

- record: grafanaorg:avg_reconciliation_duration
  expr: |
    rate(observability_operator_grafana_organization_reconciliation_duration_seconds_sum[5m]) /
    rate(observability_operator_grafana_organization_reconciliation_duration_seconds_count[5m])

- record: grafanaorg:error_rate
  expr: |
    rate(observability_operator_grafana_organization_errors_total[5m])
```

These queries provide comprehensive monitoring coverage for your GrafanaOrganization resources and can be used in dashboards, alerts, and operational runbooks. 