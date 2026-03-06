[![CircleCI](https://dl.circleci.com/status-badge/img/gh/giantswarm/observability-operator/tree/main.svg?style=svg)](https://dl.circleci.com/status-badge/redirect/gh/giantswarm/observability-operator/tree/main)

# observability-operator

This operator is in charge of handling the setup and configuration of the Giant Swarm observability platform.

It reconciles `cluster.x-k8s.io/Cluster` objects and provisions the full observability stack for each managed cluster: Alloy agent configuration for metrics, logs, events, and traces; gateway authentication secrets for Mimir, Loki, and Tempo; and the `observability-bundle` App CR that deploys Alloy onto the workload cluster.

It also manages Grafana organizations (via the `GrafanaOrganization` CRD), datasource provisioning, SSO settings, Alertmanager configuration, and Grafana dashboard provisioning.

## Features

### Alertmanager config provisioning

It will look for kubernetes `Secrets` and use them as alertmanager configs if they meet these criteria:
- a label `observability.giantswarm.io/kind: alertmanager-config`
- an annotation or label `observability.giantswarm.io/tenant` set to the tenant that the alertmanager config should be loaded in.

Current limitations:
- no support for merging configs for the same tenant. Creating 2 secrets for the same tenant will result in an unexpected behavior as the operator could unknowingly replace one with the other.
- each alertmanager config belongs to one and only one tenant

### Grafana dashboards provisioning

It will look for kubernetes `ConfigMaps` and use them as dashboards if they meet these criteria:
- a label `app.giantswarm.io/kind: "dashboard"`
- an annotation or label `observability.giantswarm.io/organization` set to the organization the dasboard should be loaded in.

Current limitations:
- no support for folders
- each dashboard belongs to one and only one organization

## Getting started

Get the code and build it via:

```bash
git clone https://github.com/giantswarm/observability-operator.git
cd observability-operator
make
```

See `make help` for help.

If you want to run the operator locally against an existing cluster, you can use `make local` which will use `hack/bin/run-local.sh` to setup a local instance for the operator.

## Architecture

The operator runs four Kubernetes controllers:

- **ClusterMonitoringReconciler** — watches `cluster.x-k8s.io/Cluster` objects and provisions the full observability stack per managed cluster: Alloy agent config for metrics (→Mimir), logs (→Loki), events (→Loki), and traces (→Tempo); gateway authentication secrets; and the `observability-bundle` App CR.
- **GrafanaOrganizationReconciler** — watches `GrafanaOrganization` CRs and manages Grafana organizations, per-org datasource configuration, and SSO settings.
- **AlertmanagerController** — watches `Secrets` labelled `observability.giantswarm.io/kind: alertmanager-config` and pushes assembled Alertmanager configuration to Mimir Alertmanager.
- **DashboardController** — watches `ConfigMaps` labelled `app.giantswarm.io/kind: dashboard` and provisions dashboards into the correct Grafana organization.

Validation webhooks cover alertmanager config secrets, dashboard configmaps, and `GrafanaOrganization` CRs (including conversion between API versions v1alpha1 and v1alpha2).

Observability features are controlled per-cluster via labels on the `Cluster` object. Metrics, logging, and tracing are opt-out (enabled by default); network monitoring and KEDA authentication are opt-in.

## Credits

This operator was built using [`kubebuilder`](https://book.kubebuilder.io/quick-start.html).
