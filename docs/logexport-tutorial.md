# Exporting logs to S3

A walk through one `LogExport`, from the resource that configures it to the objects that appear in
your bucket and what is inside them.

For which selectors are accepted, see [LogExport selectors](logexport-selectors.md).

## 1. Provide the credentials

A `LogExport` is namespaced, and works in **your own namespace on the management cluster** — your
organization namespace, or any namespace you can write to. Nothing has to be requested from Giant
Swarm. This walkthrough uses `my-namespace`:

```bash
kubectl create namespace my-namespace
```

The exporter authenticates to S3 with the AWS SDK's default credential chain, and static credentials
are the only option that works today. Put them in a Secret **in the same namespace as the
`LogExport`** — the reference is by name only and cannot cross namespaces.

Write the file with the keys empty, so nothing secret is ever typed on the command line:

```bash
cat > aws.env <<'EOF'
AWS_ACCESS_KEY_ID=
AWS_SECRET_ACCESS_KEY=
EOF
```

Fill the two values in with an editor — passing them as arguments would leave them in your shell
history. Then create the Secret from the file and remove the file:

```bash
kubectl -n my-namespace create secret generic logexport-aws-credentials --from-env-file=aws.env
rm aws.env
```

The two keys are fixed.

`roleARN` is for cross-account delivery, and **is not usable yet**: the exporter would assume the role
with its own workload identity, and nothing currently gives it one. Until that exists, a
`credentialsRef` is required.

## 2. Apply the LogExport

```yaml
apiVersion: observability.giantswarm.io/v1alpha1
kind: LogExport
metadata:
  name: audit
  namespace: my-namespace
spec:
  selector: '{scrape_job="audit-logs"}'
  destination:
    type: s3
    s3:
      bucket: acme-audit-archive
      region: us-east-1
      prefix: audit
      credentialsRef:
        name: logexport-aws-credentials
```

Creating the resource switches the export on; deleting it switches it off again. There is no separate
feature flag.

Several `LogExport`s may write to the same bucket — each one renders its own exporter, and object
names never collide. They may each name a `credentialsRef` too, as long as those resolve to the same
credentials, which is the usual shape for two destinations in one AWS account. Credentials that
disagree are refused, naming both resources: static credentials reach the exporter as environment
variables, so there is only one set to go round.

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
bytes. Check which behaviour your client has before assuming a download is compressed.

## 4. What is inside an object

### `format: otlp` (the default)

One OTLP document per object. The log line is untouched, in `body.stringValue`, and the labels the
platform attached to it arrive as record attributes.

This is the format to use when the line does not describe itself — container output being the case
that matters. Here is one such line, `[INFO] 10.0.0.1:38000 - 38341 "A IN cluster.local. udp"`, and
everything the object holds around it:

```json
{"resourceLogs":[{"resource":{},"scopeLogs":[{"scope":{},"logRecords":[{
  "timeUnixNano":"1788513842581338799",
  "body":{"stringValue":"[INFO] 10.0.0.1:38000 - 38341 \"A IN cluster.local. udp\""},
  "attributes":[
    {"key":"cluster_id","value":{"stringValue":"wc01"}},
    {"key":"namespace","value":{"stringValue":"kube-system"}},
    {"key":"pod","value":{"stringValue":"coredns-7d8f4b6c9-x2vlq"}},
    {"key":"container","value":{"stringValue":"coredns"}},
    {"key":"scrape_job","value":{"stringValue":"kubernetes-pods"}},
    {"key":"organization","value":{"stringValue":"acme"}},
    {"key":"log.file.path","value":{"stringValue":"/var/log/pods/kube-system_coredns-7d8f4b6c9-x2vlq_.../coredns/0.log"}},
    {"key":"loki.attribute.labels","value":{"stringValue":"cluster_id,namespace,pod,container,scrape_job,organization"}}
  ]}]}]}]}
```

Shown formatted; in the object it is one line with no trailing newline.

The body on its own means nothing — the pod, namespace and container that produced it are in the
attributes, and only `otlp` carries them. Note that the collector's own bookkeeping rides along too:
`log.file.path` and `loki.attribute.labels` sit alongside the useful labels.

Reading it takes more work: a consumer walks `resourceLogs` → `scopeLogs` → `logRecords`, and parses
`body.stringValue` as its own document where the line happens to be JSON.

### `format: raw`

```yaml
    s3:
      bucket: acme-audit-archive
      region: us-east-1
      format: raw
```

The log lines alone, newline-delimited and **unaltered** — no envelope, no added fields.

A Kubernetes audit event comes out exactly as the API server wrote it:

```json
{
  "kind": "Event",
  "apiVersion": "audit.k8s.io/v1",
  "level": "Request",
  "auditID": "1950fdcf-5e59-48b1-928c-6f2b7c1d9a04",
  "verb": "get",
  "user": {
    "username": "system:serviceaccount:collector:observability"
  },
  "objectRef": {
    "resource": "pods",
    "namespace": "acme-app"
  },
  "annotations": {
    "authorization.k8s.io/decision": "allow",
    "authorization.k8s.io/reason": "RBAC: allowed by ClusterRoleBinding \"log-collector\""
  }
}
```

Shown formatted; in the object each record is one line.

**`raw` carries no metadata at all.** None of the labels reach the object, so a `raw` archive cannot
be traced back to a cluster, a node or a pod. A Kubernetes audit event happens to identify the user,
verb and object it concerns, so it is still useful on its own — but nothing in it says which cluster
it came from. If you need that, use `otlp`.

### Choosing

| | `otlp` | `raw` |
|---|---|---|
| Storage file format | one JSON document | newline-delimited log lines |
| Log line format | wrapped inside a JSON envelope | untouched |
| Labels | kept as attributes | dropped |
| Read log line | decode JSON and read body field | direct |
| Size | large, see below | small |

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

## 6. Reading the archive

With `format: raw`:

```bash
aws s3 cp s3://acme-audit-archive/audit/year=2026/…/logs_01a06bbb-….txt.gz - \
  | gunzip -c \
  | jq -c '{auditID, verb, user: .user.username}'
```

With `format: otlp`, unwrap the envelope and read the body. `fromjson` applies here only because an
audit event is itself JSON — a container line you would take as it comes:

```bash
aws s3 cp s3://acme-audit-archive/audit/year=2026/…/logs_01a06bbb-….json.gz - \
  | gunzip -c \
  | jq -c '.resourceLogs[].scopeLogs[].logRecords[]
           | {cluster: (.attributes[] | select(.key=="cluster_id") | .value.stringValue),
              event: (.body.stringValue | fromjson | {auditID, verb})}'
```

One caveat when loading it anywhere:

- **Delivery is at-least-once**, so the same record can appear more than once. Usually structured
  streams carry their own unique identifier to deduplicate on. For instance `auditID` on a Kubernetes
  audit event, `uid` on a Teleport one.

## 7. Removing an export

```bash
kubectl -n my-namespace delete logexport audit
```

**Objects already written stay in your bucket.** Nothing on the Giant Swarm side ever deletes them,
so the archive is yours to keep and yours to expire — set a lifecycle policy on the bucket if you
want the objects to be deleted.
