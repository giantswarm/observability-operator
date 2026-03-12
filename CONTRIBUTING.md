# Contributing to observability-operator

## Development Setup

### Stack

- **Language**: Go — see `go.mod` for the current version
- **Framework**: [kubebuilder](https://book.kubebuilder.io/) / controller-runtime
- **Build**: `make` (devctl-generated Makefile)
- **CI**: CircleCI (main pipeline) + GitHub Actions (alertmanager integration tests, version checks)
- **Tests**: Ginkgo v2 (controllers), testify table-driven (pkg/), BATS (alertmanager routes), Kind + Mimir (alertmanager integration)

### Prerequisites

- Go (version in `go.mod`)
- `make`
- `kubectl` + a kubeconfig pointing at a cluster (only needed for `make run-local` — unit/integration tests run without a cluster via envtest)
- [cert-manager](https://cert-manager.io/) on the cluster (only needed for full local runs, for webhook TLS)

### Getting Started

```sh
git clone https://github.com/giantswarm/observability-operator.git
cd observability-operator
go build ./...                    # verify it compiles
make test                         # unit + integration tests
make run-local                    # run operator locally against an existing management cluster
```

See `make help` for all available targets.

## Testing

### Test commands

```bash
make test                                    # unit + controller integration tests (main target)
make generate-golden-files                   # regenerate golden files for monitoring config templates
make tests-alertmanager-routes               # BATS alertmanager route tests (no cluster needed)
make tests-alertmanager-integration-setup    # spin up Kind + Mimir
make tests-alertmanager-integrations         # full alertmanager integration tests
make coverage-html                           # generate HTML coverage report
```

### Test categories

| Category | Location | Runner | Notes |
|---|---|---|---|
| Unit | `./pkg/...`, `./internal/predicates` | `go test` / Ginkgo | Pure Go, no K8s |
| Controller integration | `./internal/controller` | Ginkgo + envtest | Runs a real kube-apiserver |
| Webhook integration | `./internal/webhook/...` | Ginkgo + envtest | Same envtest setup |
| Alertmanager routes | `tests/alertmanager-routes/` | BATS + `amtool` | No cluster needed |
| Alertmanager integration | `tests/alertmanager-integration/` | Kind + Mimir | Full stack |

### Golden files

The cluster monitoring controller renders Alloy config templates. Expected outputs are stored as golden files and compared on every test run. After changing a template:

```bash
make generate-golden-files
```

### Alertmanager route tests

BATS tests in `tests/alertmanager-routes/` validate routing logic using `amtool` built from the same Grafana fork commit. No cluster needed.

### Tool setup

All tools (Ginkgo, envtest, controller-gen) are managed automatically:

```bash
make install-tools    # install all dev tools
make setup-envtest    # download Kubernetes test binaries for envtest
```

**Troubleshooting:**
- `KUBEBUILDER_ASSETS not set` — run `make setup-envtest`
- Generated files out of date — run `make generate-all`, then `make verify-generate`
- Tools not found — run `make clean-tools && make install-tools`

### Manual end-to-end testing

Deploy your branch to a testing installation and suspend Flux reconciliation for the `observability-operator` app before running manual tests.

```bash
make manual-testing INSTALLATION=<installation-name>
```

This creates a workload cluster and verifies the metric collector is deployed correctly. After the script completes, wait ~10 minutes and verify:

- Grafana dashboards show data from all workload clusters (especially `ollyoptest`)
- No unexpected new alerts on the `Alerts timeline` dashboard

Then revert the Flux suspension.

## Coding Conventions

### Controllers (`internal/controller/`)

- No business logic in controllers — reconcile methods delegate entirely to `pkg/` services
- Errors collected with `errors.Join()` so all independent tasks run before returning
- Finalizer added first before any mutation, removed last after all cleanup
- Context propagated via `log.FromContext(ctx)` / `log.IntoContext(ctx, logger)`
- Error wrapping: `fmt.Errorf("context: %w", err)` throughout, never swallowed
- Rate limiter on cluster controller: exponential backoff 1s→5min

### CRD / API

- `GrafanaOrganization` is cluster-scoped; v1alpha2 is the storage version
- Conversion webhook bridges v1alpha1 ↔ v1alpha2; update both directions when fields change
- TenantID: `^[a-zA-Z_][a-zA-Z0-9_]{0,149}$` — enforced in webhook; `__mimir_cluster` is forbidden

### Alertmanager

- Uses Grafana's fork of prometheus-alertmanager (`go.mod` replace directive) for Mimir compatibility
- Config pushed via Mimir Alertmanager API, not written to a file
- BATS tests validate routes against `amtool` built from the same fork commit

### Go Patterns

- Modern Go: `any`, `slices`, `cmp.Compare()`, `errors.Join()` — already used throughout
- Interfaces defined at the consumer side (e.g. `grafanaclient.GrafanaClientGenerator`)
- Tests: table-driven with testify for `pkg/`, Ginkgo suites for controllers, golden files for template output

### Metrics

Operator metrics are defined in `pkg/metrics/metrics.go` and registered via `init()`. To add a new metric:

1. Declare it as a `prometheus.NewGaugeVec` / `CounterVec` etc. in `metrics.go`
2. Register it in the `init()` `metrics.Registry.MustRegister(...)` call
3. Record it in the appropriate controller reconcile path
4. Add it to the metrics table in `README.md` and relevant monitoring docs

See the [prometheus/client_golang docs](https://pkg.go.dev/github.com/prometheus/client_golang/prometheus) for the full API.

### Helm Chart

- `values.yaml` and `values.schema.json` must both be updated when adding or changing values
- Uses cert-manager for webhook TLS (`webhook/certificate.yaml`)
- No hardcoded registry — uses `.Values.image.registry`
- Standard GiantSwarm labels: `app.giantswarm.io/team: atlas`

## Submitting a PR

See [.github/pull_request_template.md](.github/pull_request_template.md) for the full review checklist.

Key points:
- Update `CHANGELOG.md` under `[Unreleased]` for every meaningful change
- Update docs if behaviour, feature flags, CRD fields, or configuration changes
- Run `make test` and `make tests-alertmanager-routes` before opening the PR
