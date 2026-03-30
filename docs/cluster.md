# Per-Cluster Observability Configuration

The operator supports enabling/disabling observability features per workload cluster via labels and annotations on the `Cluster` object. These override the installation-level defaults set in Helm values.

## Feature flags

Set these labels on the `cluster.x-k8s.io/Cluster` object to control which features are active for that cluster.

| Feature | Label | Default |
|---|---|---|
| Metrics | `observability.giantswarm.io/monitoring` | enabled |
| Logs | `observability.giantswarm.io/logging` | enabled |
| Traces | `observability.giantswarm.io/tracing` | enabled |
| Network monitoring | `observability.giantswarm.io/network-monitoring` | disabled |
| KEDA authentication | `observability.giantswarm.io/keda-authentication` | disabled |

Both the installation-level `enabled` flag (Helm value) and the per-cluster label must allow the feature for it to be active.

### OTLP signals (installation-level opt-in)

OTLP metrics and OTLP logs are enabled at the **installation level** via Helm values, and then gated per cluster by the existing `monitoring` / `logging` labels.

| Feature | Helm value | Per-cluster gate |
|---|---|---|
| OTLP metrics (workload cluster → Mimir) | `monitoring.otlpEnabled: true` | `observability.giantswarm.io/monitoring` |
| OTLP logs (workload cluster → Loki) | `logging.otlpEnabled: true` | `observability.giantswarm.io/logging` |

When OTLP metrics is active, resource attributes (`k8s.cluster.name`, `k8s.cluster.type`, `k8s.cluster.organization`, `cloud.provider`) are automatically promoted to Mimir metric labels via `-distributor.otel-promote-resource-attributes`.

Tenant routing for all OTLP signals uses the `giantswarm.tenant` resource attribute, which is resolved from either the pod label `observability.giantswarm.io/tenant` or the `X-Scope-OrgID` HTTP header.

### Example — disable logging for a cluster

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: my-cluster
  labels:
    observability.giantswarm.io/logging: "false"
```

### Example — enable network monitoring for a cluster

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: my-cluster
  labels:
    observability.giantswarm.io/network-monitoring: "true"
```

## KEDA namespace

When KEDA authentication is enabled, the operator creates a `ClusterTriggerAuthentication` resource so KEDA `ScaledObjects` can authenticate with Mimir. By default it targets the `keda` namespace. Override it per cluster:

```yaml
metadata:
  annotations:
    observability.giantswarm.io/keda-namespace: my-keda-namespace
```

## Sharding overrides

By default, the operator configures 1 Alloy-Metrics shard per 1M time series (with a 20% scale-down threshold). These can be tuned per cluster via annotations:

| Annotation | Description |
|---|---|
| `observability.giantswarm.io/monitoring-agent-scale-up-series-count` | Time series count that triggers adding a shard |
| `observability.giantswarm.io/monitoring-agent-scale-down-percentage` | Fraction of `scaleUpSeriesCount` below which a shard is removed |

Installation-level defaults are set via `monitoring.sharding` in Helm values.

### Example

```yaml
metadata:
  annotations:
    observability.giantswarm.io/monitoring-agent-scale-up-series-count: "500000"
    observability.giantswarm.io/monitoring-agent-scale-down-percentage: "0.15"
```
