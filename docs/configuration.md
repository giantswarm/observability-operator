# Operator Configuration Reference

This document covers all Helm values for deploying the observability-operator. See `helm/observability-operator/values.yaml` for defaults.

## `managementCluster`

Metadata about the management cluster. Required for the operator to function correctly.

| Value | Description |
|---|---|
| `managementCluster.baseDomain` | Base domain of the management cluster (e.g. `example.gigantic.io`) |
| `managementCluster.clusterIssuer` | cert-manager ClusterIssuer used for webhook TLS |
| `managementCluster.customer` | Customer identifier |
| `managementCluster.insecureCA` | Skip CA verification — for development only |
| `managementCluster.name` | Cluster name |
| `managementCluster.pipeline` | Pipeline identifier (e.g. `stable`, `testing`) |
| `managementCluster.region` | Region identifier |

## `alerting`

Controls Alertmanager config delivery, Grafana alerting, and Cronitor heartbeats.

| Value | Description |
|---|---|
| `alerting.enabled` | Enable alerting support |
| `alerting.alertmanagerURL` | URL of the Mimir Alertmanager API |
| `alerting.grafanaAddress` | Grafana instance address |
| `alerting.slackAPIToken` | Slack API token for the default Slack receiver |
| `alerting.cronitorHeartbeatPingKey` | Cronitor ping key for heartbeat alerts |
| `alerting.cronitorHeartbeatManagementKey` | Cronitor management key |
| `alerting.teams` | List of team-specific configs (see below) |

**Team configuration** — used to configure per-team PagerDuty routing:

```yaml
alerting:
  teams:
    - name: my-team
      pagerdutyToken: <token>
```

## `monitoring`

Controls metrics collection from workload clusters via Alloy agents.

| Value | Description |
|---|---|
| `monitoring.enabled` | Enable monitoring at the installation level |
| `monitoring.sharding.scaleUpSeriesCount` | Time series count per Alloy shard (default: `1000000`) |
| `monitoring.sharding.scaleDownPercentage` | Scale-down threshold as a fraction of `scaleUpSeriesCount` (default: `0.20`) |
| `monitoring.wal.truncateFrequency` | How often to truncate the WAL (default: `15m`) |

**Queue configuration** — all fields are optional; Alloy defaults apply when not set:

| Value | Description |
|---|---|
| `monitoring.queueConfig.batchSendDeadline` | Max time samples wait before sending |
| `monitoring.queueConfig.capacity` | Samples buffered per shard |
| `monitoring.queueConfig.maxBackoff` | Maximum retry delay |
| `monitoring.queueConfig.maxSamplesPerSend` | Maximum samples per send |
| `monitoring.queueConfig.maxShards` | Maximum concurrent shards |
| `monitoring.queueConfig.minBackoff` | Initial retry delay |
| `monitoring.queueConfig.minShards` | Minimum concurrent shards |
| `monitoring.queueConfig.retryOnHttp429` | Retry on HTTP 429 responses |
| `monitoring.queueConfig.sampleAgeLimit` | Max age of samples to send (`0s` = disabled) |

Per-cluster overrides for sharding are also available via Cluster annotations. See [cluster.md](cluster.md).

> **Note for local development:** All `queueConfig` fields can also be set via CLI flags when running the operator binary directly (e.g. `make run-local`): `--monitoring-queue-config-batch-send-deadline`, `--monitoring-queue-config-capacity`, `--monitoring-queue-config-max-shards`, etc. CLI flags take precedence over Helm values if explicitly set.

### Tuning guidelines

**High throughput:** increase `maxShards` and `maxSamplesPerSend`; consider increasing `capacity`.

**Low latency:** decrease `batchSendDeadline`; increase `minShards`.

**Memory constrained:** decrease `capacity` and `maxShards`.

**Unreliable network:** increase `maxBackoff`; enable `retryOnHttp429`; set a `sampleAgeLimit` to avoid accumulating stale data.

### Monitoring queue health

These metrics are emitted by the Alloy agent on each workload cluster and visible on the `Prometheus / Remote Write` Grafana dashboard:

| Metric | Description |
|---|---|
| `prometheus_remote_storage_shards` | Current shard count |
| `prometheus_remote_storage_shards_desired` | Desired shard count (what the autoscaler is targeting) |
| `prometheus_remote_storage_samples_pending` | Samples pending in queues |
| `prometheus_remote_storage_shard_capacity` | Capacity per shard |

## `tracing`

| Value | Description |
|---|---|
| `tracing.enabled` | Enable distributed tracing support |

## `logging`

Controls log collection and related features.

| Value | Description |
|---|---|
| `logging.enabled` | Enable logging support |
| `logging.enableNodeFiltering` | Filter logs by node |
| `logging.enableNetworkMonitoring` | Enable network monitoring (opt-in at install level) |
| `logging.defaultNamespaces` | Default namespaces to collect logs from |
| `logging.includeEventsFromNamespaces` | Namespaces to include for Kubernetes events |
| `logging.excludeEventsFromNamespaces` | Namespaces to exclude for Kubernetes events |

## `webhook`

Controls the validating webhook configuration.

| Value | Description |
|---|---|
| `webhook.enabled` | Enable webhook validation globally |
| `webhook.validatingWebhooks.alertmanagerConfig.enabled` | Validate alertmanager config secrets |
| `webhook.validatingWebhooks.dashboardConfigMap.enabled` | Validate dashboard ConfigMaps |
| `webhook.validatingWebhooks.grafanaOrganization.enabled` | Validate GrafanaOrganization CRs |

## `operator`

Resource and security configuration for the operator pod. Defaults are appropriate for most deployments.

```yaml
operator:
  resources:
    requests:
      cpu: 100m
      memory: 100Mi
    limits:
      cpu: 100m
      memory: 150Mi
```
