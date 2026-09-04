# Exporting logs to S3

A walk through one `LogExport`, from the resource that configures it to the objects that appear in
your bucket and what is inside them.

For which selectors are accepted, see [LogExport selectors](logexport-selectors.md).

## 1. Provide the credentials

A `LogExport` is namespaced, and works in **your own namespace on the management cluster** — your
organization namespace, or any namespace you can write to. Nothing has to be requested from Giant
Swarm.

The exporter authenticates to S3 with the AWS SDK's default credential chain. For static credentials,
put them in a Secret **in the same namespace as the `LogExport`** — the reference is by name only and
cannot cross namespaces:

```bash
kubectl -n org-acme create secret generic archive-credentials \
  --from-literal=AWS_ACCESS_KEY_ID=AKIA... \
  --from-literal=AWS_SECRET_ACCESS_KEY=...
```

The two keys are fixed. To authenticate by role instead, omit `credentialsRef` and set `roleARN` —
preferable where you can use it, since no key has to be handed over or rotated.

## 2. Apply the LogExport

```yaml
apiVersion: observability.giantswarm.io/v1alpha1
kind: LogExport
metadata:
  name: audit
  namespace: org-acme
spec:
  selector: '{scrape_job="audit-logs"}'
  destination:
    type: s3
    s3:
      bucket: acme-audit-archive
      region: us-east-1
      prefix: audit
      credentialsRef:
        name: archive-credentials
```

Creating the resource switches the export on; deleting it switches it off again. There is no separate
feature flag, and one `LogExport` per destination is fine.

`format` is not set above, so it defaults to `otlp`. Section 4 shows what each format produces.

## 3. What appears in the bucket

Objects are written under the `prefix`, partitioned by time:

```
audit/year=2026/month=09/day=04/hour=09/minute=24/logs_01a06bbb-75ac-72c3-8324-608193725750.json.gz
└─┬─┘ └───────────────────┬──────────────────────┘ └───────────────────┬───────────────────────────┘
prefix          partition, always UTC                       logs_<uuidv7>.<ext>.gz
```

The extension follows the format: `.json.gz` for `otlp`, `.txt.gz` for `raw`.

Three things worth knowing about the layout:

- **The partition is upload time, not event time.** A record produced at 09:23 but uploaded at 09:24
  lands under `minute=24`. A query that filters only on the partition will miss late arrivals, so
  filter on the event's own timestamp as well.
- **The timestamp is always UTC**, regardless of where the exporter runs.
- **The name is a UUIDv7**, so object names sort in creation order and never collide.

Each object holds a batch, not a single record, so the number of objects does not track the number of
log lines.

### Object metadata

```json
{"ContentType": "application/octet-stream", "ContentEncoding": "gzip", "ContentLength": 1191}
```

`ContentEncoding: gzip` matters more than it looks. Some S3 clients see it and decompress `.gz`
objects transparently, so the bytes you get may already be plain text; others hand you the compressed
bytes. Athena treats a `ContentEncoding`-marked object differently from a plain `.gz` too. Check which
behaviour your client has before assuming a download is compressed.

## 4. What is inside an object

### `format: otlp` (the default)

One OTLP document per object. The log line is untouched, in `body.stringValue`, and the labels the
platform attached to it arrive as record attributes:

```json
{"resourceLogs":[{"resource":{},"scopeLogs":[{"scope":{},"logRecords":[{
  "timeUnixNano":"1788513842581338799",
  "body":{"stringValue":"{\"kind\":\"Event\",\"apiVersion\":\"audit.k8s.io/v1\",\"level\":\"Request\",…}"},
  "attributes":[
    {"key":"cluster_id","value":{"stringValue":"wc01"}},
    {"key":"node","value":{"stringValue":"ip-10-0-0-1.eu-west-1.compute.internal"}},
    {"key":"namespace","value":{"stringValue":"acme-app"}},
    {"key":"resource","value":{"stringValue":"pods"}},
    {"key":"scrape_job","value":{"stringValue":"audit-logs"}},
    {"key":"organization","value":{"stringValue":"acme"}},
    {"key":"provider","value":{"stringValue":"capa"}},
    {"key":"cluster_type","value":{"stringValue":"workload_cluster"}}
  ]}]}]}]}
```

Shown formatted; in the object it is one line with no trailing newline.

This is the format to use when the line does not describe itself. Container output is the case that
matters — `[INFO] 10.0.0.1:38000 - 38341 "A IN …"` means little without the pod, namespace and
container that produced it, and only `otlp` carries them.

Two things to expect:

- **It also carries the collector's own attributes** — `log.file.path`, `log.file.name` and
  `loki.attribute.labels` appear alongside the useful ones.
- **Reading it takes more work.** A consumer walks `resourceLogs` → `scopeLogs` → `logRecords`, then
  parses `body.stringValue` as its own document. In Athena that is three levels of unnesting plus a
  `json_parse`.

### `format: raw`

```yaml
    s3:
      bucket: acme-audit-archive
      region: us-east-1
      format: raw
```

The log lines alone, newline-delimited and **unaltered** — no envelope, no added fields. `jq`, Athena
and DuckDB read the decompressed object directly, and in Athena it is a plain
`org.openx.data.jsonserde.JsonSerDe` table over the lines.

A Kubernetes audit event comes out exactly as the API server wrote it:

```json
{"kind":"Event","apiVersion":"audit.k8s.io/v1","level":"Request","auditID":"1950fdcf-5e59-48b1-928c-…",
 …,"annotations":{"authorization.k8s.io/decision":"allow","authorization.k8s.io/reason":"RBAC: allowed by
 ClusterRoleBinding \"log-collector\" of ClusterRole \"log-collector\" to ServiceAccount \"collector/observability\""}}
```

**`raw` carries no metadata at all.** None of the labels reach the object, so a `raw` archive cannot
be traced back to a cluster, a node or a pod. A Kubernetes audit event happens to identify the user,
verb and object it concerns, so it is still useful on its own — but nothing in it says which cluster
it came from. If you need that, use `otlp`.

### Choosing

| | `otlp` | `raw` |
|---|---|---|
| Log line | untouched, inside an envelope | untouched, alone |
| Labels | kept as attributes | dropped |
| Object | one JSON document | newline-delimited lines |
| Reading | unnest, then parse the body | direct |
| Size | larger, see below | smaller |

`otlp` runs roughly one and a half to five times the compressed size of `raw` for the same records.
The multiplier depends on how long the lines are and how many land in one object: the envelope
repeats per record, so short lines and small batches pay most, and large batches of similar records
compress well. Measure with your own data before sizing a bucket.

## 5. Picking a selector

The streams on an installation differ in whether a line means anything on its own, which is what
decides the format:

| `scrape_job` | Shape | Suggested format |
|---|---|---|
| `audit-logs` | Kubernetes audit events, JSON | either — `raw` if you want a plain table and can live without the cluster id |
| `teleport.giantswarm.io` | Teleport audit events, JSON | either, same trade |
| `system-logs` | journald records, JSON | either |
| `kubernetes-events` | logfmt, not JSON | `otlp` |
| `kubernetes-pods` | container output, mostly not JSON | `otlp` |

For the last two, `raw` produces an archive nothing can be attributed to.

## 6. Reading the archive

With `format: raw`:

```bash
aws s3 cp s3://acme-audit-archive/audit/year=2026/…/logs_01a06bbb-….txt.gz - \
  | gunzip -c \
  | jq -c '{auditID, verb, user: .user.username}'
```

With `format: otlp`, unwrap the envelope and parse the body:

```bash
aws s3 cp s3://acme-audit-archive/audit/year=2026/…/logs_01a06bbb-….json.gz - \
  | gunzip -c \
  | jq -c '.resourceLogs[].scopeLogs[].logRecords[]
           | {cluster: (.attributes[] | select(.key=="cluster_id") | .value.stringValue),
              event: (.body.stringValue | fromjson | {auditID, verb})}'
```

Two caveats when loading it anywhere:

- **Delivery is at-least-once.** The same record can appear more than once, so deduplicate on
  `auditID` for Kubernetes audit events, or `uid` for Teleport.
- **Partition on upload time, event time in the record.** See the layout note above.

## 7. Removing an export

```bash
kubectl -n org-acme delete logexport audit
```

**Objects already written stay in your bucket.** Nothing on the Giant Swarm side ever deletes them,
so the archive is yours to keep and yours to expire — set a lifecycle policy on the bucket if you
want the objects aged out.
