[![CircleCI](https://dl.circleci.com/status-badge/img/gh/giantswarm/observability-operator/tree/main.svg?style=svg)](https://dl.circleci.com/status-badge/redirect/gh/giantswarm/observability-operator/tree/main)

# observability-operator

Brain of the Giant Swarm observability platform: provisions and configures the full observability stack (metrics, logs, traces, alerts, dashboards) for every managed cluster.

It handles four responsibilities:

- **Cluster monitoring** — reconciles `cluster.x-k8s.io/Cluster` objects and provisions the full observability stack per managed cluster: Alloy agent config for metrics, logs, events, and traces; gateway authentication secrets for Mimir, Loki, and Tempo; and the `observability-bundle` App CR that deploys Alloy onto the workload cluster.
- **Grafana organizations** — manages Grafana organizations via the `GrafanaOrganization` CRD: creates orgs, configures per-org datasources, and sets up SSO role mappings.
- **Alertmanager configuration** — assembles and pushes tenant Alertmanager configs to Mimir Alertmanager from labeled Kubernetes Secrets.
- **Dashboard provisioning** — provisions Grafana dashboards from labeled Kubernetes ConfigMaps, including folder hierarchy management.

## Project Structure

```
observability-operator/
├── api/                                # CRD type definitions (kubebuilder)
│   ├── v1alpha1/                       # GrafanaOrganization v1alpha1
│   └── v1alpha2/                       # GrafanaOrganization v1alpha2 (storage version)
├── cmd/
│   └── main.go                         # Operator entry point
├── config/                             # Kubebuilder manifests (CRDs, RBAC, webhooks)
├── docs/                               # Feature documentation
├── helm/
│   └── observability-operator/         # Helm chart for deployment
│       └── files/alertmanager/         # Alertmanager config templates
├── internal/
│   ├── controller/                     # Reconcilers (agentcredential, cluster, dashboard, alertmanager, grafanaorganization)
│   ├── mapper/                         # Watch event mappers (dashboard, organization)
│   ├── predicates/                     # Event filter predicates
│   └── webhook/                        # Validating & conversion webhooks
│       ├── v1/                         # Secret & ConfigMap validators
│       ├── v1alpha1/                   # GrafanaOrganization v1alpha1 validator
│       ├── v1alpha2/                   # GrafanaOrganization v1alpha2 validator
│       └── validation/                 # Shared validation logic
├── pkg/
│   ├── agent/                          # Alloy ConfigMap/Secret repository
│   │   ├── collectors/                 # Per-signal config builders:
│   │   │   ├── metrics/                # Alloy metrics config + KEDA auth objects
│   │   │   ├── logs/                   # Alloy logs + network monitoring config
│   │   │   └── events/                 # Alloy Kubernetes events config
│   │   └── common/                     # Shared Alloy config keys
│   ├── alerting/
│   │   ├── alertmanager/               # Assembles + pushes config to Mimir Alertmanager API
│   │   └── heartbeat/                  # Cronitor heartbeat monitor management
│   ├── auth/                           # Gateway auth secrets (Mimir, Loki, Tempo credentials)
│   ├── bundle/                         # observability-bundle App/HelmRelease CR reconciliation
│   ├── common/                         # Shared utilities (labels, tenancy, organization helpers)
│   ├── config/                         # Operator config struct from Helm values / CLI flags
│   ├── domain/                         # Domain types (orgs, dashboards, folders, clusters)
│   ├── grafana/                        # Grafana HTTP client (orgs, datasources, dashboards, SSO)
│   │   └── client/                     # HTTP client, TLS, credential management
│   ├── metrics/                        # Prometheus metrics declarations (all custom metrics)
│   └── monitoring/
│       ├── mimir/                      # Mimir querier (head series)
│       └── sharding/                   # Per-cluster agent sharding logic
├── tests/
│   ├── alertmanager-routes/            # BATS route tests + Go integration tests
│   ├── alertmanager-integration/       # Kind + Mimir setup for integration tests
│   ├── ats/                            # Acceptance test suite
│   └── bats/                           # BATS test framework
└── CHANGELOG.md                        # Keep a Changelog format
```

## Architecture

Five controllers run in a single binary:

| Controller | Watches | Does |
|---|---|---|
| `ClusterReconciler` | `cluster.x-k8s.io/Cluster` | Provisions Alloy config (metrics→Mimir, logs→Loki, events→Loki, traces→Tempo), declares `AgentCredential` CRs per backend, observability-bundle App CR, Cronitor heartbeat |
| `AgentCredentialReconciler` | `AgentCredential` (v1alpha1) | Mints per-credential basic-auth Secrets and aggregates htpasswd entries into per-backend gateway Secrets. Only registered when `--auth-mode=basicAuth` |
| `GrafanaOrganizationReconciler` | `GrafanaOrganization` (v1alpha2) | Creates/updates Grafana orgs, datasources, SSO settings |
| `AlertmanagerController` | `Secret` with `observability.giantswarm.io/kind: alertmanager-config` | Assembles + pushes Alertmanager config to Mimir Alertmanager |
| `DashboardController` | `ConfigMap` with `app.giantswarm.io/kind: dashboard` | Provisions dashboards into Grafana orgs via API |

Validating webhooks enforce constraints on alertmanager config secrets, dashboard configmaps, `GrafanaOrganization`, and `AgentCredential` CRs. A conversion webhook handles `GrafanaOrganization` version conversion between v1alpha1 and v1alpha2.

## Features

### Alertmanager config provisioning

The operator watches for `Secrets` with the following labels and uses them as Alertmanager configs:

- `observability.giantswarm.io/kind: alertmanager-config`
- `observability.giantswarm.io/tenant: <tenant>` (label or annotation)

One secret per tenant. See [docs/alertmanager.md](docs/alertmanager.md) for full usage and examples.

### Grafana dashboard provisioning

The operator watches for `ConfigMaps` with the following labels and provisions them as Grafana dashboards:

- `app.giantswarm.io/kind: dashboard`
- `observability.giantswarm.io/organization: <org-name>` (label or annotation)
- `observability.giantswarm.io/folder: <path>` (optional, supports nested hierarchy e.g. `team/subteam`)

See [docs/dashboards.md](docs/dashboards.md) for full usage and examples.

### Grafana organization management

The operator manages Grafana organizations via the `GrafanaOrganization` CRD. See [docs/grafana-organization.md](docs/grafana-organization.md) for CRD reference and examples.

### Agent credentials

The operator mints one basic-auth credential per `(telemetry agent, observability backend)` pair, declared via the `AgentCredential` CRD. Three CRs are auto-created per CAPI cluster (metrics/logs/traces); additional CRs can be created for agents that are not backed by a managed cluster. See [docs/agent-credential.md](docs/agent-credential.md) for CRD reference, auth modes, and examples.

### Per-cluster feature flags

Observability features are controlled per-cluster via labels on the `Cluster` object:

| Feature | Label | Model | Default |
|---|---|---|---|
| Metrics | `observability.giantswarm.io/monitoring` | opt-out | enabled |
| Logging | `observability.giantswarm.io/logging` | opt-out | enabled |
| Tracing | `observability.giantswarm.io/tracing` | opt-out | enabled |
| Network monitoring | `observability.giantswarm.io/network-monitoring` | opt-in | disabled |
| KEDA auth | `observability.giantswarm.io/keda-authentication` | opt-in | disabled |

The KEDA operator namespace can be overridden per-cluster via the `observability.giantswarm.io/keda-namespace` annotation (default: `keda`).

See [docs/cluster.md](docs/cluster.md) for full details including per-cluster sharding and queue tuning.

## Getting Started

The operator is deployed via the Helm chart in `helm/observability-operator/`.

For local development and contributing, see [CONTRIBUTING.md](CONTRIBUTING.md).

## Observability

The operator exposes the following metrics (prefix: `observability_operator_`):

| Metric | Description |
|---|---|
| `observability_operator_grafana_organization_info` | GrafanaOrganization info (labels: `name`, `display_name`, `org_id`, `status`) |
| `observability_operator_grafana_organization_tenants` | Info gauge, 1 per (tenant, org) — use `count(...) by (org_id)` for tenant count per org (labels: `name`, `org_id`) |
| `observability_operator_alertmanager_routes` | Route count per tenant (label: `tenant`) |
| `observability_operator_mimir_head_series_query_errors_total` | Counter of Mimir query errors |
| `observability_operator_monitored_cluster_info` | Info gauge, 1 per active monitored cluster (labels: `cluster_name`, `cluster_namespace`) |
| `observability_operator_grafana_api_errors_total` | Counter of Grafana API errors (label: `operation`) |
| `observability_operator_mimir_alertmanager_api_errors_total` | Counter of Mimir Alertmanager API errors (label: `operation`) |
| `observability_operator_ruler_api_errors_total` | Counter of ruler API errors (label: `operation`) |
| `observability_operator_agent_credential_info` | Info gauge, 1 per active AgentCredential (labels: `namespace`, `name`, `backend`, `agent_name`) |
| `observability_operator_agent_credential_reconcile_errors_total` | Counter of AgentCredential reconcile errors (labels: `backend`, `step`) |

Self-monitoring via PodMonitor at `helm/observability-operator/templates/pod-monitor.yaml`. Alerts, dashboards, and runbooks live in the `prometheus-rules` and `dashboards` repositories.

See [docs/metrics.md](docs/metrics.md) for the full metrics reference.

## Documentation

| Doc | Description |
|---|---|
| [docs/grafana-organization.md](docs/grafana-organization.md) | GrafanaOrganization CRD usage and examples |
| [docs/agent-credential.md](docs/agent-credential.md) | AgentCredential CRD usage and auth modes |
| [docs/alertmanager.md](docs/alertmanager.md) | Alertmanager config secret usage and examples |
| [docs/dashboards.md](docs/dashboards.md) | Dashboard provisioning and folder support |
| [docs/cluster.md](docs/cluster.md) | Per-cluster observability feature flags and sharding overrides |
| [docs/metrics.md](docs/metrics.md) | Operator metrics reference and example queries |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Development setup, testing, and coding conventions |

## Credits

This operator was built using [`kubebuilder`](https://book.kubebuilder.io/quick-start.html).
