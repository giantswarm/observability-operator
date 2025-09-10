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

## Configuration Monitoring

### Tenants
```promql
# Total tenants across all organizations
sum(observability_operator_grafana_organization_tenants)

# Organizations with most tenants
topk(5, observability_operator_grafana_organization_tenants)

# Organizations with single tenant
observability_operator_grafana_organization_tenants == 1
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

## Dashboard Queries

### Summary Stats for Dashboard
```promql
# For stat panels
sum(observability_operator_grafana_organizations_total)
observability_operator_grafana_organizations_total{status="active"}
observability_operator_grafana_organizations_total{status="pending"}
observability_operator_grafana_organizations_total{status="error"}

# For tables
observability_operator_grafana_organization_info
```

## Custom Recording Rules

You can create custom recording rules for commonly used queries:

```yaml
# Example recording rules
- record: grafanaorg:total_by_status
  expr: sum(observability_operator_grafana_organizations_total) by (status)
```

These queries provide comprehensive monitoring coverage for your GrafanaOrganization resources and can be used in dashboards, alerts, and operational runbooks.
