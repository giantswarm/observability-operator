# observability-operator — Product & Usability Plan

## Context

This plan consolidates findings from a full audit of the operator's capability gaps, API surface, documentation, and test coverage. The goal is to make the operator a complete, self-explanatory product for both Giant Swarm internal teams and external adopters — covering missing observability features, reducing adoption friction, and improving reliability.

---

## Tier 1 — Missing Capabilities

### 1. Exemplars — Done (PR #737, pending merge)
**Why:** Tempo and Mimir both support exemplars. The Grafana UI already renders trace-to-metrics and metrics-to-traces drill-downs. But `send_exemplars` is never set in the Alloy remote write pipeline, so the correlation story is broken despite the infrastructure being in place.

**What was done:**
- Added `send_exemplars = true` to the remote write block in the Alloy config template
- Exposed `monitoring.exemplars.enabled` (default `true`) as an opt-out Helm value wired through the full stack
- Updated all 12 golden test fixtures

---

### 2. Native Histograms
**Why:** Prometheus native histograms are more accurate (no fixed buckets), lower cardinality, and directly supported by Mimir and Alloy. Classic histograms are a ceiling on percentile accuracy.

**What to do:**
- Add `native_histogram_bucket_limit` and `convert_classic_histograms_to_nhcb` config to the scrape blocks in the metrics template
- Add `send_native_histograms: true` to the remote write block
- Expose `monitoring.nativeHistograms.enabled` in Helm values + schema
- Update `pkg/config/monitoring.go`
- Regenerate golden files

**Files:** `pkg/agent/collectors/metrics/templates/alloy-config.alloy.template`, `pkg/config/monitoring.go`, `helm/observability-operator/values.yaml`, `helm/observability-operator/values.schema.json`

---

### 3. OTLP for Metrics and Logs — Done (PR #738 + PR #739, pending merge)
**Why:** The events collector already had a full OTLP receiver/exporter pipeline for traces (gRPC 4317, HTTP 4318). Applications using the OTel SDK to push metrics or logs had no endpoint. The OTLP story was half-complete.

**What was done (PR #738 — fix/alloy-deprecate-nonsensitive-and-fix-logging-template):**
- Fixed `nonsensitive()` → `convert.nonsensitive()` in events, logs, and metrics Alloy templates (6 occurrences)
- Fixed unclosed `remote.kubernetes.secret "credentials"` block in logging template
- Regenerated all golden test files

**What was done (PR #739 — feat/otlp-unified-events-collector):**
- Extended events collector to a unified OTLP gateway: single `otelcol.receiver.otlp "default"` handles traces + metrics + logs
- Added `otelcol.processor.k8sattributes`, `otelcol.processor.transform`, `otelcol.processor.batch` shared across all signals
- Added per-tenant `otelcol.exporter.otlphttp` for Mimir (`/otlp/v1/metrics`, HTTP) and Loki (`/otlp/v1/logs`, HTTP)
- Traces continue to use `otelcol.exporter.otlp` (gRPC) to Tempo — unchanged
- Exposed `monitoring.otlp.enabled` / `logging.otlp.enabled` flags (Helm values, CLI flags, config structs)
- Added `MetricsAuthManager` to events service for Mimir OTLP credential handling
- Added 5 new golden file test cases (MC/WC × OTLP metrics/logs/all-signals)

**What was done (PR #595 on giantswarm/shared-configs):**
- Added `/otlp/v1/metrics` to Mimir HTTPRoute (`additionalRules` with X-Scope-OrgID check + `matches` catch-all 400)
- Added `/otlp/v1/logs` to Loki HTTPRoute `matches`

**What was done (PR #56 on giantswarm/observability-platform-api):**
- Added rules for `/otlp/v1/metrics` and `/otlp/v1/logs` to the Mimir and Loki WC write HTTPRoutes (with/without X-Scope-OrgID header enforcement)

**Verified (no changes needed):**
- `mimir-distributed` v5.8.0 nginx gateway already has `location /otlp/v1/metrics` → distributor
- `loki` v6.53.0 nginx gateway already routes `/otlp/v1/logs` → distributor
- `allow_structured_metadata` is `true` by default in Loki 3.x

#### 3a. OTLP gRPC migration (internal, transparent to users)
**Why:** Mimir and Loki only support OTLP over HTTP (their gRPC port 9095 is for their internal cluster protocol, not OTLP gRPC). Once Mimir/Loki add native OTLP gRPC support, the alloy-events exporter can switch from `otelcol.exporter.otlphttp` to `otelcol.exporter.otlp` (gRPC), connecting directly to the distributor pods and bypassing the nginx gateway. This is a pure internal efficiency improvement — no user-facing Helm values, flags, or endpoint changes required.

**What to do (when Mimir/Loki support OTLP gRPC):**
- Change `otelcol.exporter.otlphttp "{{ . }}_mimir"` → `otelcol.exporter.otlp "{{ . }}_mimir"` in `events-logger.alloy.template`; update MC endpoint to `mimir-distributor.mimir.svc:9095`
- Change `otelcol.exporter.otlphttp "{{ . }}_loki"` → `otelcol.exporter.otlp "{{ . }}_loki"`; update MC endpoint to `loki-write.loki.svc:9095` (or distributor port TBD)
- Remove the per-signal HTTP path from secrets (URL keys); endpoint becomes a hardcoded constant for MC, same gRPC port for WC
- Regenerate golden files
- No changes to Helm values, CLI flags, or external-facing routes

---

### 4. Continuous Profiling (Pyroscope)
**Why:** Profiling is the only entirely absent observability pillar. Grafana ships Pyroscope natively; Giant Swarm already runs the rest of the Grafana stack.

**What to do:**
- Create `pkg/agent/collectors/profiling/` package following the existing collector pattern (service.go, configmap.go, template)
- Template uses `pyroscope.scrape` and `pyroscope.write` Alloy components
- Add a Pyroscope datasource to the GrafanaOrganization reconciler in `pkg/grafana/datasources_default.go`
- Add `apps.AlloyProfiling` toggle to `pkg/bundle/service.go`
- Wire into `ClusterMonitoringReconciler` in `internal/controller/cluster_controller.go`
- Expose `profiling.enabled` in Helm values + schema + `pkg/config/`
- Add golden file tests

**Files:** New `pkg/agent/collectors/profiling/`, `pkg/grafana/datasources_default.go`, `pkg/bundle/service.go`, `internal/controller/cluster_controller.go`, `pkg/config/`

---

## Tier 2 — Usability & API

### 5. Quickstart + Label Reference Documentation — Done (PR #731, pending merge)
**Why:** The README had a "TODO: Fill this out" for Architecture. There was no quickstart, no label/annotation reference, and no example manifests. This was the single biggest blocker for new adopters.

**What was done:**
- Rewrote README.md with architecture table, feature flags table, links to all docs
- New CONTRIBUTING.md with dev setup, test categories, coding conventions
- New `docs/alertmanager.md`, `docs/dashboards.md`, `docs/grafana-organization.md`
- New `docs/cluster.md` — all per-cluster feature flags, opt-in/opt-out model, examples
- New `docs/configuration.md` — full Helm values reference, queue config tuning
- New `docs/metrics.md` — all 4 operator metrics with example queries
- Fixed invalid PromQL, updated GrafanaOrganization example to v1alpha2

---

### 6. Label Namespace Migration — Done (PR #727, pending merge)
**Why:** Three label prefixes were in use (`giantswarm.io/*`, `observability.giantswarm.io/*`, `app.giantswarm.io/*`) with explicit TODOs in code.

**What was done:**
- All cluster feature toggle labels migrated from `giantswarm.io/*` to `observability.giantswarm.io/*`
- Full backwards compatibility: new label checked first, falls back to legacy if absent
- Both a primary constant and a `Legacy*` constant exposed per config file
- Companion docs PRs: giantswarm/docs#3016 and giantswarm/giantswarm#35929

---

### 7. Decouple Network Monitoring from Logging — Done (PR #733, pending merge)
**Why:** `logging.enableNetworkMonitoring` was a confusing API. Beyla eBPF network flow collection has nothing to do with logging conceptually.

**What was done:**
- Operator flag renamed from `--logging-enable-network-monitoring` to `--monitoring-enable-network-monitoring`
- Helm value moved from `logging.enableNetworkMonitoring` → `monitoring.enableNetworkMonitoring`
- Old value kept as a deprecated alias for backward compatibility

**Note:** The original plan called for a standalone top-level `networkMonitoring` section; the implementation moved it under `monitoring.enableNetworkMonitoring` instead. This achieves the conceptual decoupling from logging and is sufficient — a further rename to a dedicated top-level key could be a future cleanup.

---

### 8. Getting Started Guide (living doc)
**Why:** The new docs cover each feature individually but there's no single "zero to working observability in 5 minutes" narrative. New adopters need a guided path: install the operator → enable monitoring on a cluster → verify metrics flow → enable logging → enable tracing → configure alertmanager. This doc will grow as new features ship (exemplars, native histograms, OTLP, profiling).

**What to do:**
- Create `docs/getting-started.md` with a step-by-step walkthrough:
  1. Prerequisites and install
  2. Enable monitoring on a cluster (set `observability.giantswarm.io/monitoring: "true"` label, verify Alloy ConfigMap + metrics in Mimir)
  3. Enable logging (`observability.giantswarm.io/logging: "true"`, verify logs in Loki)
  4. Enable tracing (`observability.giantswarm.io/tracing: "true"`, verify traces in Tempo)
  5. Enable network monitoring (`monitoring.enableNetworkMonitoring: true` Helm value)
  6. Configure alertmanager (create Secret with config, verify in Mimir)
  7. Create a GrafanaOrganization and verify datasources
- Add a **label quick-reference table** at the top: label → what it enables → default → example value
- Add "What's next" links to each feature's dedicated doc
- Update this doc whenever a new feature is added (exemplars opt-out, native histograms, OTLP, profiling)

**Files:** New `docs/getting-started.md`, `README.md` (add prominent link)

---

### 9. Alertmanager Fork Limitation Documentation
**Why:** The webhook returns "Grafana fork limitations" in error messages but there is no documentation on what is or isn't supported. Users hit cryptic failures.

**What to do:**
- Add `docs/alertmanager/supported-features.md` listing which Alertmanager features are supported/unsupported in the Grafana fork (`github.com/grafana/prometheus-alertmanager`)
- Update the webhook error message in `internal/webhook/v1/alertmanager_config_secret_webhook.go` to link to the doc
- Note the fork version in the doc so it can be updated when the fork is bumped

**Files:** New `docs/alertmanager/supported-features.md`, `internal/webhook/v1/alertmanager_config_secret_webhook.go`

---

### 9. Configurable Defaults — Avoid Hardcoded Constants
**Why:** Secret names, namespace names, and gateway identifiers are hardcoded as `const` values throughout the codebase (e.g. gateway secret name used for Grafana TLS). This makes it impossible for operators running in non-standard configurations to override them without forking. Supporting custom config like gateway secret name and namespaces is a key adoption enabler.

**What to do:**
- Audit `pkg/` and `internal/` for all `const` values that represent names of external resources (secret names, namespace names, service names, gateway names)
- Move these into Helm values with the current const as the default (e.g. `grafana.gatewayTLSSecretName`, `grafana.namespace`)
- Wire through config structs (`pkg/config/`) so they reach the reconcilers
- Update `values.yaml` and `values.schema.json` accordingly
- Good starting points: Gateway API TLS secret name (see PR #736), Mimir/Loki endpoint namespaces, Alertmanager service name

**Files:** `pkg/config/`, `helm/observability-operator/values.yaml`, `helm/observability-operator/values.schema.json`, scattered `internal/` and `pkg/` files

---

## Tier 3 — Reliability & Correctness

### 10. Alertmanager Config Deletion Lifecycle — Done (PR #726, pending merge)
**Why:** Deleting a tenant's alertmanager Secret left config permanently in Mimir. The controller had an explicit TODO for this.

**What was done:**
- Alertmanager config secrets now receive a finalizer so Mimir config is deleted when the secret is removed
- `AlertmanagerReconciler` rewritten with proper create/delete split
- `Service` interface extended with `DeleteForTenant`; concrete HTTP implementation added
- Full Ginkgo integration tests (envtest + mock service) covering the complete reconciliation lifecycle
- `TestConfigureFromSecret` and `TestDeleteForTenant` unit tests in `pkg/alertmanager`
- All four controllers' `reconcileDelete` aligned to the early-return finalizer pattern
- Log messages homogenised across all controllers

---

### 11. Mimir Ruler Cleanup on Cluster Deletion
**Why:** Recording and alerting rules pushed to Mimir Ruler are never removed when a cluster is deleted. Orphaned rules accumulate.

**What to do:**
- Add `DeleteRules(tenant string)` to the metrics service or a new `pkg/mimir/` client
- Call from `ClusterMonitoringReconciler`'s deletion path in `internal/controller/cluster_controller.go`

**Files:** `internal/controller/cluster_controller.go`, `pkg/agent/collectors/metrics/service.go` (or new `pkg/mimir/`)

---

### 12. Webhook Integration Tests
**Why:** The three webhooks have no integration tests. The validation logic is non-trivial (tenant existence, YAML parsing, duplicate detection).

**What to do:**
- Add table-driven tests to `internal/webhook/v1/alertmanager_config_secret_webhook_test.go`: valid config, invalid YAML, missing label, unknown tenant, duplicate tenant
- Mirror for dashboard ConfigMap and GrafanaOrganization webhooks
- Use `envtest` (already used in controller tests) for tenant existence checks

**Files:** `internal/webhook/v1/alertmanager_config_secret_webhook_test.go`, `internal/webhook/v1/dashboard_configmap_webhook_test.go`, `internal/webhook/v1alpha2/grafanaorganization_webhook_test.go`

---

### 13. `pkg/alertmanager` Service Interface — Done (PR #725, superseded by PR #726)
**Why:** `Service` was a concrete struct with no interface, making it impossible to mock in unit tests.

**What was done (via PR #726 which absorbed this):**
- `Service` interface extracted covering `ConfigureFromSecret` and `DeleteForTenant`
- Concrete struct renamed to `service` (unexported); `New()` returns the interface
- Controller updated to depend on the interface; mock injected in tests

---

## Priority Order (remaining work)

| # | Item | Effort | Impact |
|---|---|---|---|
| 1 | Getting started guide | Low | High |
| 2 | Configurable defaults (avoid const) | Medium | High |
| 3 | Native histograms | Low | Medium |
| ~~4~~ | ~~OTLP metrics/logs~~ | ~~High~~ | ~~High~~ — Done (PR #738, #739, shared-configs #595, platform-api #56) |
| 4 | OTLP gRPC migration (internal, no user impact) | Low | Low — defer until Mimir/Loki support OTLP gRPC natively |
| 5 | Webhook integration tests | Medium | Medium |
| 6 | Mimir ruler cleanup | Medium | Medium |
| 7 | Alertmanager fork docs | Low | Medium |
| 8 | Continuous profiling | High | High |

---

## Verification

- `make test` — all unit tests pass
- `make generate-golden-files` — golden files up to date after any template change
- `make tests-alertmanager-routes` — BATS route tests pass
- `make tests-alertmanager-integration-setup && make tests-alertmanager-integrations` — integration tests pass
- Manual: apply example manifests from new `docs/examples/` against a real cluster and verify each feature works end-to-end
