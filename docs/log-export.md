# Log export

Archives Kubernetes API audit logs and Teleport audit logs to an object store, so a
customer can keep their own long-term copy independently of Loki's retention.

Product context: [giantswarm/giantswarm#37251](https://github.com/giantswarm/giantswarm/issues/37251).

## How it works

Log export does not read from Loki. Nothing in the ecosystem consumes Loki as a
stream, and querying it would need checkpointing, pagination and gap handling.
Instead the write path is **teed** at the gateway:

```
                              ┌─ primary (unaffected) ─→ loki-gateway ─→ Loki
producers ─→ Envoy HTTPRoute ─┤
                              └─ RequestMirror (fire-and-forget) ─→ alloy-logexporter ─→ object store
```

An Envoy Gateway `HTTPRequestMirrorFilter` duplicates Loki push requests to
`alloy-logexporter`, which selects the lines the customer asked for and writes them
as gzipped, newline-delimited JSON.

`alloy-logexporter` is shipped to every management cluster by
[`management-cluster-bases`](https://github.com/giantswarm/management-cluster-bases)
collections in an **inert** state: it renders no workloads at all until this operator
writes its values. Enabling an installation is therefore a configuration change here,
not a GitOps change per installation.

## Enabling it

Two independent things have to be true:

1. **This operator writes the config** — `--log-export-enabled=true` on the
   installation, and the management cluster's `Cluster` object must not carry
   `observability.giantswarm.io/log-export: false`.
2. **The mirror filter is in place** on the Loki write `HTTPRoute`. Without it
   `alloy-logexporter` runs and receives nothing.

The exporter only ever runs on the **management cluster**. The mirror is configured
once at the gateway, so a workload cluster instance would receive no traffic, and the
operator will not configure one.

### Flags

| Flag | Default | Description |
|---|---|---|
| `--log-export-enabled` | `false` | Enable log export on this installation |
| `--log-export-namespace` | `giantswarm` | Namespace of the `alloy-logexporter` HelmRelease. Must match it, because a HelmRelease resolves `valuesFrom` references in its own namespace only |
| `--log-export-bucket` | | Destination bucket |
| `--log-export-region` | `us-east-1` | Destination bucket region |
| `--log-export-prefix` | `audit` | Key prefix for exported objects |
| `--log-export-endpoint` | | Object store endpoint. Empty uses the AWS default for the region; set it for an S3-compatible store |
| `--log-export-force-path-style` | `false` | Address the bucket as a path rather than a subdomain. Generally required by S3-compatible stores |

### Credentials

Set `LOGEXPORT_AWS_ACCESS_KEY_ID` and `LOGEXPORT_AWS_SECRET_ACCESS_KEY` in the
operator's environment. Both are required; the operator refuses to configure the
exporter without them, rather than deploying something that silently drops
everything.

When the destination is reached with IRSA instead, no credentials Secret is needed —
the exporter's `envFrom` reference is optional and the AWS SDK picks up the
ServiceAccount role.

## What lands in the bucket

Objects are gzipped, newline-delimited raw audit JSON:

```
<prefix>/year=2026/month=08/day=03/hour=15/minute=57/logs_<uuidv7>.txt.gz
```

Partitioning is Hive-style, so Glue partition projection works directly. Each line is
the original audit event with one field added:

```json
{"kind":"Event","apiVersion":"audit.k8s.io/v1",...,"gs_cluster_id":"my-cluster"}
```

`gs_cluster_id` is injected because a Kubernetes audit event carries no cluster
identifier of its own, and the export format keeps only the log line — every Loki
label is dropped.

## Limitations

State these to customers up front; each one otherwise gets reported as a bug.

- **No backfill.** Export starts when it is enabled. There is no history before that.
- **Best-effort delivery.** Mirroring is fire-and-forget, which is exactly why it
  cannot break log ingestion, and also why Envoy will not retry a mirrored request.
  If `alloy-logexporter` is down, mirrored requests are dropped silently while the
  primary write succeeds. Measured on a testing installation: a five-minute exporter
  outage lost 100% of the export for its duration, one contiguous window, with no
  recovery.
- **An object store outage longer than the exporter's timeout loses data
  permanently.** The write-ahead log carries buffered records through a short outage
  and across a pod restart, but the awss3 exporter's `retry_on_failure` is not
  exposed by Alloy and is disabled, so the request timeout is the durability window.
- **Duplicates are expected.** At-least-once, measured at roughly 4%. Dedupe on
  `auditID` (Kubernetes audit) or `uid` (Teleport).
- **Loki and the bucket will disagree.** Loki dedupes identical entries on read; the
  bucket does not. The same window shows fewer entries in Grafana than in the bucket.
- **Logs submitted via OTLP are not captured.** The mirrored route also matches
  `/otlp/v1/logs`, but the exporter only serves the Loki push API, so those requests
  are answered with 404 and never archived. Audit logs are unaffected, because the
  per-cluster agents use the push API.
- **Partitioning is by upload time, not event time.** A record delayed by the queue
  or a retry lands in the partition for when it was written, so a partition-only
  filter can miss late arrivals.
- **`ContentEncoding: gzip`** is set on the objects, so some S3 clients transparently
  decompress them even though the key ends in `.gz`.

## Monitoring it

The signals that show loss, none of which are visible from object presence alone:

| Metric | Meaning |
|---|---|
| `otelcol_exporter_enqueue_failed_log_records_total` | queue overflow — records dropped before upload |
| `otelcol_exporter_send_failed_log_records_total` | upload given up on |
| `otelcol_exporter_queue_size` | destination backing up |
| `loki_source_api_request_duration_seconds_count{status_code="503"}` | the exporter shedding load |
| `loki_process_dropped_lines_total{reason="not_selected"}` | expected drops from the selection — should **not** alert |

On the Envoy side the mirror backend gets its own cluster, named after the route rule
rather than the backend, e.g.
`envoy_cluster_name="httproute/loki/loki-gateway/rule/0-mirror-0"`. Two failure modes
need different alert shapes:

- **Exporter saturated** → the mirror cluster's 5xx rate rises. Alertable directly.
- **Exporter absent** → the mirror cluster's series **stop being emitted**. An alert
  written `rate(...) == 0` never fires, because an absent series yields no result.
  Use `absent()`, or compare against the primary rule's rate.
