@README.md
@CONTRIBUTING.md
@.github/pull_request_template.md
@docs/configuration.md
@docs/grafana-organization.md
@docs/alertmanager.md
@docs/dashboards.md
@docs/cluster.md
@docs/metrics.md

# CLAUDE.md — observability-operator

## Key Files

```
api/v1alpha1/                               — hub type for v1alpha1↔v1alpha2 conversion
api/v1alpha2/grafanaorganization_types.go   — storage version (tenants, RBAC, displayName)
helm/observability-operator/files/alertmanager/ — Alertmanager config templates (not in pkg/)
tests/alertmanager-routes/                  — BATS route tests + Go integration tests
tests/alertmanager-integration/             — Kind + Mimir setup for integration tests
CHANGELOG.md                               — Keep a Changelog format, updated with every meaningful change
```

## Key packages (`pkg/`)

```
pkg/agent/                      — Alloy ConfigMap/Secret repository; collector sub-packages:
pkg/agent/collectors/metrics/   — renders Alloy metrics config templates + KEDA auth objects
pkg/agent/collectors/logs/      — renders Alloy logs + network monitoring config templates
pkg/agent/collectors/events/    — renders Alloy Kubernetes events config templates
pkg/alertmanager/               — assembles + pushes Alertmanager config to Mimir Alertmanager API
pkg/alerting/heartbeat/         — Cronitor heartbeat monitor management
pkg/auth/                       — gateway auth secret management (Mimir, Loki, Tempo credentials)
pkg/bundle/                     — observability-bundle App/HelmRelease CR reconciliation
pkg/config/                     — operator config struct populated from Helm values / CLI flags
pkg/domain/                     — domain types for orgs, dashboards, folders, clusters, monitoring
pkg/grafana/                    — Grafana HTTP client wrappers (orgs, datasources, dashboards, SSO)
pkg/metrics/                    — Prometheus metric declarations (all custom metrics defined here)
pkg/monitoring/                 — finalizer constant only (MonitoringFinalizer)
pkg/common/                     — shared utilities (labels, tenancy, organization helpers)
```
