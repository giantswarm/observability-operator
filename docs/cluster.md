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

When a backend is enabled for a cluster, the cluster controller declares an `AgentCredential` CR per backend under the cluster's namespace: `<cluster-name>-observability-{metrics,logs,traces}`. The [AgentCredential controller](agent-credential.md) mints the Secret and aggregates the htpasswd entries into the per-backend gateway Secrets. Disabling a backend on a cluster deletes the corresponding CR and removes its entry from the aggregated htpasswd.

### OTLP signals

OTLP metrics and OTLP logs are always active when the corresponding feature is enabled — there is no separate opt-in flag.

| Feature | Activation |
|---|---|
| OTLP metrics (workload cluster → Mimir) | Always active when `observability.giantswarm.io/monitoring` is enabled |
| OTLP logs (workload cluster → Loki) | Always active when `observability.giantswarm.io/logging` is enabled |

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

## vCenter metrics

On vSphere and Cloud Director installations, the management cluster Alloy agent collects provider metrics from vCenter with `otelcol.receiver.vcenter`. The metrics go to the default tenant through `otelcol.exporter.prometheus`. Workload cluster agents do not collect these metrics.

Collection is disabled by default. Set `monitoring.vcenter.enabled` to `true` in the operator Helm values to enable it. Enable it only on installations that provide the credentials described below.

The operator does not create the credentials. Provide a Secret named `alloy-vcenter-credentials` in the `kube-system` namespace of the management cluster, with these keys:

| Key | Description |
|---|---|
| `vcenter-endpoint` | URL of the vCenter API endpoint. The `http://` or `https://` scheme is mandatory, for example `https://vcenter.example.com` |
| `vcenter-username` | Username for vCenter authentication |
| `vcenter-password` | Password for vCenter authentication |
| `vcenter-insecure-skip-verify` | `"true"` to skip TLS verification, any other value to verify |

All keys are mandatory. Alloy fails to load its configuration if a key is absent.

Alloy removes leading and trailing whitespace from all values except the password, which stays a secret. Make sure the password has no trailing newline. Use `stringData` in the Secret manifest, or `kubectl create secret generic --from-literal`, to prevent this.

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

## Remote write queue tuning

The Alloy-Metrics remote write queue (`prometheus.remote_write`'s `queue_config`) is **installation-level only** — there is no per-cluster annotation for it. Set it via `monitoring.queueConfig` in the operator's Helm values, or via the matching `--monitoring-queue-config-*` CLI flags when running the binary directly.

Note that shards come in two independent flavours:

| Concept | Scope | Controlled by |
|---|---|---|
| Alloy-Metrics replicas | Pods in the StatefulSet, computed from head series | `monitoring.sharding` + the annotations above |
| `max_shards` / `min_shards` | Remote write concurrency *inside each pod*, autoscaled by Alloy | `monitoring.queueConfig` |

The defaults match upstream Prometheus/Alloy (`capacity: 10000`, `maxSamplesPerSend: 2000`, `maxShards: 50`), with `sampleAgeLimit: 30m` as a Giant Swarm addition. When tuning, keep `capacity` well above `maxSamplesPerSend` — if `capacity` is the smaller of the two, Prometheus gives each shard a single-batch channel and the queue loses its buffer.

## Mimir rules liveness probe

With observability-bundle 3.6.0 or later, the operator adds a liveness probe to `alloy-metrics`. The probe fails when a `mimir.rules.kubernetes` component stays unhealthy while the Mimir ruler answers. Kubernetes then restarts Alloy. The probe succeeds when its script is missing, for example when a values layer replaces `alloy.alloy.mounts.extra` or `alloy.controller.volumes.extra`. This probe is **installation-level only**. Set it via `monitoring.mimirRulesLivenessProbe` in the operator's Helm values, or via the matching `--monitoring-mimir-rules-liveness-probe-*` CLI flags.

| Value | Default | Description |
|---|---|---|
| `enabled` | `true` | `false` removes the probe. The upstream Alloy chart liveness probe applies. |
| `httpTimeout` | `20s` | Timeout of each HTTP request the probe sends. Sets `HTTP_TIMEOUT`. |
| `probeTimeout` | `50s` | Total budget of one probe run. The probe succeeds when exceeded. Sets `PROBE_TIMEOUT`. |
| `healthAgeMargin` | `60s` | Margin added to the component `sync_interval`. The probe only fails for a component health older than both. Sets `HEALTH_AGE_MARGIN`. |
| `unhealthyMessagePattern` | `^(error GET [^ ]*/rules: server returned HTTP status (429\|5[0-9]{2}) \|Get "[^"]*/rules": )` | Extended regular expression matched against the component health message. Sets `UNHEALTHY_MESSAGE_PATTERN`. |
| `mimirReadyPath` | `/prometheus/config/v1/rules` | Path the probe requests on the Mimir ruler to check it answers. Sets `MIMIR_READY_PATH`. |
| `initialDelaySeconds` | `60` | Liveness probe `initialDelaySeconds` |
| `periodSeconds` | `60` | Liveness probe `periodSeconds` |
| `timeoutSeconds` | `60` | Liveness probe `timeoutSeconds` |
| `failureThreshold` | `3` | Liveness probe `failureThreshold` |

The operator fails at startup unless:

- `httpTimeout`, `probeTimeout` and `healthAgeMargin` are whole seconds from `1s` to `999999s`. `1500ms` is rejected.
- `httpTimeout <= probeTimeout`.
- `probeTimeout + 2s <= timeoutSeconds`. One probe run takes up to `probeTimeout` plus a second.
- `unhealthyMessagePattern` is not empty and compiles as a Go regular expression. The probe script runs `grep -E`. A pattern Go accepts can still be invalid for `grep -E`, for example `\d` or `(?i)`. The probe then never fails.
- `mimirReadyPath` starts with `/`.
- `initialDelaySeconds` is from `0` to `2147483647`. `periodSeconds`, `timeoutSeconds` and `failureThreshold` are from `1` to `2147483647`.

A disabled probe skips these checks.
