# CLAUDE.md — observability-operator

## Purpose
Brain of the Giant Swarm observability platform: provisions and configures the full observability stack (metrics, logs, traces, alerts, dashboards) for every managed cluster.

## Architecture

Four controllers in a single binary (see README.md for public-facing overview):

| Controller | Watches | Does |
|---|---|---|
| `ClusterMonitoringReconciler` | `cluster.x-k8s.io/Cluster` | Provisions Alloy config (metrics→Mimir, logs→Loki, events→Loki, traces→Tempo), gateway auth secrets, observability-bundle App CR, Cronitor heartbeat |
| `GrafanaOrganizationReconciler` | `GrafanaOrganization` (v1alpha2) | Creates/updates Grafana orgs, datasources, SSO settings |
| `AlertmanagerController` | `Secret` `observability.giantswarm.io/kind: alertmanager-config` | Assembles + pushes Alertmanager config to Mimir Alertmanager |
| `DashboardController` | `ConfigMap` `app.giantswarm.io/kind: dashboard` | Provisions dashboards into Grafana orgs via API |

Webhooks (`internal/webhook/`): alertmanager config secrets, dashboard configmaps, GrafanaOrganization CRs (validation + v1alpha1↔v1alpha2 conversion).

## Stack
- Language: Go 1.25 (toolchain 1.26)
- Build: `make` (devctl-generated), kubebuilder scaffolding
- CI: CircleCI + GitHub Actions (alertmanager integration tests, alertmanager version check)
- Tests: Ginkgo (controllers), testify table-driven (pkg/), BATS (alertmanager routes), Kind + Mimir integration tests

## Key Files

```
api/v1alpha1/                               — hub type for v1alpha1↔v1alpha2 conversion
api/v1alpha2/grafanaorganization_types.go   — storage version (tenants, RBAC, displayName)
helm/observability-operator/files/alertmanager/ — Alertmanager config templates (not in pkg/)
tests/alertmanager-routes/                  — BATS route tests (separate from integration tests)
tests/alertmanager-integration/             — Kind + Mimir setup for integration tests
CHANGELOG.md                               — Keep a Changelog format, updated with every meaningful change
```

## Development Workflow

```sh
make build                        # build binary
make test                         # unit tests (Ginkgo + testify)
make generate-golden-files        # regenerate golden files for monitoring config templates
make tests-alertmanager-routes    # BATS alertmanager route tests (no cluster needed)
make tests-alertmanager-integration-setup  # spin up Kind + Mimir Alertmanager
make tests-alertmanager-integrations       # full alertmanager integration tests
make run-local                    # run operator locally via hack/bin/run-local.sh
make coverage-html                # generate HTML coverage report
```

Golden files: `pkg/agent/collectors/metrics/testdata/` — regenerate with `UPDATE_GOLDEN_FILES=true go test ./...`.

## Patterns & Conventions

### Controllers
- No business logic in `internal/controller/` — reconcile delegates to `pkg/` services
- Errors collected with `errors.Join()` so all independent tasks run before returning
- Finalizer added first before any mutation, removed last after all cleanup
- Context propagated via `log.FromContext(ctx)` / `log.IntoContext(ctx, logger)`
- Error wrapping: `fmt.Errorf("context: %w", err)` throughout
- Rate limiter on cluster controller: exponential backoff 1s→5min

### CRD / API
- `GrafanaOrganization` is cluster-scoped, v1alpha2 is storage version
- Conversion webhook bridges v1alpha1 ↔ v1alpha2
- TenantID: `^[a-zA-Z_][a-zA-Z0-9_]{0,149}$` — enforced in webhook; `__mimir_cluster` is forbidden

### Feature gating (labels on Cluster objects)
| Feature | Label | Model | Default |
|---|---|---|---|
| Metrics | `giantswarm.io/monitoring` | opt-out | true |
| Logging | `giantswarm.io/logging` | opt-out | true |
| Tracing | `giantswarm.io/tracing` | opt-out | true |
| Network monitoring | `giantswarm.io/network-monitoring` | opt-in | false |
| KEDA auth | `giantswarm.io/keda-authentication` | opt-in | false |

### Alertmanager
- Uses Grafana's fork of prometheus-alertmanager (go.mod replace) for Mimir compat
- Config pushed via Mimir Alertmanager API (not a file)
- BATS tests validate routes against `amtool` built from the same fork commit

### Go patterns
- Modern Go: `any`, `slices`, `cmp.Compare()`, `errors.Join()` — already used throughout
- Interfaces defined at the consumer side (e.g. `grafanaclient.GrafanaClientGenerator`)
- Tests: table-driven with testify, Ginkgo suites for controllers, golden files for template output
- Run `go tool modernize ./...` to detect outdated patterns

### Helm chart
- `values.yaml` and `values.schema.json` must both be updated when adding or changing values
- Uses cert-manager for webhook TLS (`webhook/certificate.yaml`)
- PodMonitor at `templates/pod-monitor.yaml` for operator self-metrics
- No hardcoded registry — uses `.Values.image.registry`
- Standard GiantSwarm labels: `app.giantswarm.io/team: atlas`

## PR Review Checklist

**Every PR:**
- [ ] `CHANGELOG.md` updated under `[Unreleased]` (skip only for pure refactors with no user-visible effect)
- [ ] Docs updated if the change affects operator behaviour, feature flags, CRD fields, or configuration (README Features section, any relevant runbooks)

**Go controllers:**
- [ ] Reconcile loop delegates to `pkg/`, no direct API calls in controller
- [ ] Errors wrapped with `fmt.Errorf("context: %w", err)`, not swallowed
- [ ] All independent tasks use `errors.Join()` pattern
- [ ] Finalizer added first, removed last
- [ ] New `pkg/` packages have tests (testify table-driven or Ginkgo)
- [ ] Feature flags respected: check both install-level `Enabled` AND per-cluster label
- [ ] RBAC annotations updated if new resources touched (`//+kubebuilder:rbac:...`)

**CRD changes:**
- [ ] Conversion webhook updated for both directions if v1alpha1/v1alpha2 fields change
- [ ] Webhook validation updated for new constraints
- [ ] `zz_generated.deepcopy.go` regenerated (`make generate`)

**Alertmanager:**
- [ ] Golden files updated if Alertmanager config template changes
- [ ] BATS tests cover new route scenario
- [ ] `make tests-alertmanager-routes` passes locally

**Helm:**
- [ ] `values.yaml` updated with the new value (with sensible default)
- [ ] `values.schema.json` updated to match (type, description, required if appropriate)
- [ ] Resource requests/limits set on all containers
- [ ] Image tag not hardcoded
- [ ] Alertmanager template changes validated with `make validate-alertmanager-config`

## Observability

**Metrics exposed** (prefix: `observability_operator_`):
- `observability_operator_grafana_organization_info` (labels: `name`, `display_name`, `org_id`, `status`) — status: active/pending/error
- `observability_operator_grafana_organization_tenants` (labels: `name`, `org_id`)
- `observability_operator_alertmanager_routes` (label: `tenant`)
- `observability_operator_mimir_head_series_query_errors_total` — counter

**Self-monitoring:** PodMonitor at `helm/observability-operator/templates/pod-monitor.yaml`

**Alerts / Dashboards / Runbooks:** In `prometheus-rules` repo (`helm/prometheus-rules/templates/platform/atlas/`) and `dashboards` repo.
