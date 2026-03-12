# Per-Cluster Observability Configuration

The operator supports enabling/disabling observability features per workload cluster via labels and annotations on the `Cluster` object. These override the installation-level defaults set in Helm values.

## Feature flags

Set these labels on the `cluster.x-k8s.io/Cluster` object to control which features are active for that cluster.

| Feature | Label | Model | Default |
|---|---|---|---|
| Metrics | `giantswarm.io/monitoring` | opt-out | enabled |
| Logging | `giantswarm.io/logging` | opt-out | enabled |
| Tracing | `giantswarm.io/tracing` | opt-out | enabled |
| Network monitoring | `giantswarm.io/network-monitoring` | opt-in | disabled |
| KEDA authentication | `giantswarm.io/keda-authentication` | opt-in | disabled |

**Opt-out** — the feature is enabled by default; set the label to `"false"` to disable it.
**Opt-in** — the feature is disabled by default; set the label to `"true"` to enable it.

Both the installation-level `enabled` flag (Helm value) and the per-cluster label must allow the feature for it to be active.

### Example — disable logging for a cluster

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: my-cluster
  labels:
    giantswarm.io/logging: "false"
```

### Example — enable network monitoring for a cluster

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: my-cluster
  labels:
    giantswarm.io/network-monitoring: "true"
```

## KEDA namespace

When KEDA authentication is enabled, the operator creates a `ClusterTriggerAuthentication` resource so KEDA `ScaledObjects` can authenticate with Mimir. By default it targets the `keda` namespace. Override it per cluster:

```yaml
metadata:
  annotations:
    giantswarm.io/keda-namespace: my-keda-namespace
```

## Sharding overrides

By default, the operator configures 1 Alloy-Metrics shard per 1M time series (with a 20% scale-down threshold). These can be tuned per cluster via annotations:

| Annotation | Description |
|---|---|
| `observability.giantswarm.io/monitoring-agent-scale-up-series-count` | Time series count that triggers adding a shard |
| `observability.giantswarm.io/monitoring-agent-scale-down-percentage` | Fraction of `scaleUpSeriesCount` below which a shard is removed |

Installation-level defaults are set via `monitoring.sharding` in Helm values. See [configuration.md](configuration.md).

### Example

```yaml
metadata:
  annotations:
    observability.giantswarm.io/monitoring-agent-scale-up-series-count: "500000"
    observability.giantswarm.io/monitoring-agent-scale-down-percentage: "0.15"
```
