# Exporting logs to S3

A walk through one `LogExport`, from the resource that configures it to the objects that appear in
your bucket and what is inside them.

For which selectors are accepted, see [LogExport selectors](logexport-selectors.md).

## 1. Give the exporter access to your bucket

A `LogExport` is namespaced, and works in **your own namespace on the management cluster** — your
organization namespace, or any namespace you can write to. Nothing has to be requested from Giant
Swarm. This walkthrough uses `my-namespace`:

```bash
kubectl create namespace my-namespace
```

There are two ways to authenticate. They cannot be mixed on one installation: see
[Authentication is installation-wide](#authentication-is-installation-wide).

### Option A: a role in your AWS account

On AWS management clusters the exporter has its own identity, the IAM role
`giantswarm-<installation>-alloy-logexporter` in the management cluster's account. That role can
do one thing: assume a role whose name starts with **`giantswarm-logexport-`**, in any account. Read
its ARN from the exporter's ServiceAccount:

```bash
kubectl -n monitoring get serviceaccount alloy-logexporter \
  -o jsonpath='{.metadata.annotations.eks\.amazonaws\.com/role-arn}'
```

In the account that owns the bucket, create a role named `giantswarm-logexport-<anything>` that
trusts it:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": { "AWS": "arn:aws:iam::<MC_ACCOUNT_ID>:role/giantswarm-<installation>-alloy-logexporter" },
      "Action": "sts:AssumeRole"
    }
  ]
}
```

Give it write access to the prefix and nothing else:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "s3:PutObject",
      "Resource": "arn:aws:s3:::acme-dns-archive/dns/*"
    }
  ]
}
```

If the bucket uses SSE-KMS, also allow `kms:GenerateDataKey` on its key.

Put the role's ARN in `roleARN`, and leave `credentialsRef` out.

Anyone who can create a `LogExport` on the installation can write into a bucket whose role trusts
the exporter. Use a dedicated bucket or prefix.

### Option B: static credentials

Put an access key in a Secret **in the same namespace as the `LogExport`**. The reference is by name
only and cannot cross namespaces.

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

The two keys are fixed. Name the Secret in `credentialsRef`, and leave `roleARN` out.

## 2. Apply the LogExport

```yaml
apiVersion: observability.giantswarm.io/v1alpha1
kind: LogExport
metadata:
  name: dns
  namespace: my-namespace
spec:
  selector: '{scrape_job="kubernetes-pods", namespace="kube-system", container="coredns"}'
  destination:
    type: s3
    s3:
      bucket: acme-dns-archive
      region: us-east-1
      prefix: dns
      format: otlp
      roleARN: arn:aws:iam::111122223333:role/giantswarm-logexport-dns
      # Option B instead:
      # credentialsRef:
      #   name: logexport-aws-credentials
```

Creating the resource switches the export on; deleting it switches it off again. There is no separate
feature flag.

`format` is set to its default, `otlp`. Section 4 shows what each format produces.

Several `LogExport`s may write to the same bucket, each rendering its own exporter.

### Authentication is installation-wide

Every `LogExport` on the installation shares one exporter process, and static credentials reach it
as environment variables. They take precedence over its own identity, so once one export sets a
`credentialsRef`, every export authenticates with that key. So:

- **Roles are per export.** With Option A, each `LogExport` names its own `roleARN`, so exports can
  write to buckets in different AWS accounts.
- **Static credentials are all or nothing.** If one `LogExport` sets a `credentialsRef`, every S3
  export on the installation must, and all must resolve to the same credentials. A mix, or a
  disagreement, is refused, naming both resources.
- An export with neither is accepted, and then fails every write: the exporter's own role has no S3
  permissions.

A write that fails authentication is dropped, not retried.

## 3. What appears in the bucket

Objects are written under the `prefix`, partitioned by time:

```
dns/year=2026/month=09/day=04/hour=09/minute=24/logs_01a06bbb-75ac-72c3-8324-608193725750.json.gz
└─┬─┘ └───────────────────┬──────────────────────┘ └───────────────────┬───────────────────────────┘
prefix          partition, always UTC                       logs_<uuidv7>.<ext>.gz
```

The extension follows the format: `.json.gz` for `otlp`, `.txt.gz` for `raw`.

Three things worth knowing about the layout:

- **The partition is upload time, not event time.** A record produced at 09:23 but uploaded at 09:24
  lands under `minute=24`. A query that filters only on the partition will miss late arrivals, so
  filter on the event's own timestamp as well.
- **The timestamp is always UTC**, regardless of where the exporter runs.
- **The name is a UUIDv7**, so object names sort in creation order and never collide — across
  exports as well, so several `LogExport`s can safely write to one bucket.

Each object holds a batch, not a single record, so the number of objects does not track the number of
log lines.

**The `prefix` is the only thing that says which `LogExport` wrote an object.** Nothing else in the
key identifies it — not the resource name, not the namespace. Two exports sharing a prefix produce
objects you cannot tell apart, so give each one its own prefix (or its own bucket) if you need to
separate them later. Where they also differ in `format`, the extension distinguishes them, but that
is a side effect rather than something to rely on.

### Object metadata

```json
{"ContentType": "application/octet-stream", "ContentEncoding": "gzip", "ContentLength": 1191}
```

`ContentEncoding: gzip` matters more than it looks. Some S3 clients see it and decompress `.gz`
objects transparently, so the bytes you get may already be plain text; others hand you the compressed
bytes. Check which behaviour your client has before assuming a download is compressed.

## 4. What is inside an object

Both formats are shown below with the same record, so the difference is the format and nothing else.
It is one of the lines the export above selects, a CoreDNS query:

```
[INFO] 10.0.0.1:38000 - 38341 "A IN cluster.local. udp"
```

### `format: otlp` (the default)

One OTLP document per object. The log line is untouched, in `body.stringValue`, and the labels the
platform attached to it arrive as record attributes:

```json
{"resourceLogs":[{"resource":{},"scopeLogs":[{"scope":{},"logRecords":[{
  "timeUnixNano":"1788513842581338799",
  "body":{"stringValue":"[INFO] 10.0.0.1:38000 - 38341 \"A IN cluster.local. udp\""},
  "attributes":[
    {"key":"cluster_id","value":{"stringValue":"wc01"}},
    {"key":"namespace","value":{"stringValue":"kube-system"}},
    {"key":"pod","value":{"stringValue":"coredns-workers-5d9f7c8b6-k4xqz"}},
    {"key":"container","value":{"stringValue":"coredns"}},
    {"key":"scrape_job","value":{"stringValue":"kubernetes-pods"}},
    {"key":"organization","value":{"stringValue":"acme"}},
    {"key":"loki.attribute.labels","value":{"stringValue":"cluster_id,namespace,pod,container,scrape_job,organization"}}
  ]}]}]}]}
```

Shown formatted; in the object it is one line with no trailing newline.

The body on its own means nothing — the pod, namespace and container that produced it are in the
attributes, and only `otlp` carries them. Note that the collector's own bookkeeping rides along too:
`loki.attribute.labels` sits alongside the useful labels. The attributes shown here are a subset; a
real record carries every label the platform attached to the stream.

Reading it takes more work: a consumer walks `resourceLogs` → `scopeLogs` → `logRecords`, and parses
`body.stringValue` as its own document where the line happens to be JSON.

### `format: raw`

The log lines alone, newline-delimited and **unaltered** — no envelope, no added fields. The same
record is the whole object content:

```
[INFO] 10.0.0.1:38000 - 38341 "A IN cluster.local. udp"
```

**`raw` carries no metadata at all.** None of the labels reach the object, so a `raw` archive cannot
be traced back to a cluster, a node or a pod.

### Choosing

| | `otlp` | `raw` |
|---|---|---|
| Storage file format | one JSON document | newline-delimited log lines |
| Log line format | wrapped inside a JSON envelope | untouched |
| Labels | kept as attributes | dropped |
| Read log line | decode JSON and read body field | direct |
| Size | large, see below | small |

We measured `otlp` compressed size to be 1.5x to 5x the compressed size of `raw` for the same
records. Measure with your own data before sizing a bucket.

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

### Useful selectors

The audit streams are the usual reason to set an export up. These four are accepted as they stand:

```
{scrape_job="audit-logs"}
{scrape_job="audit-logs"} | json | verb=~"create|update|patch|delete"
{scrape_job="teleport.giantswarm.io"}
{scrape_job="teleport.giantswarm.io"} | json | event_type="kube.request"
```

## 6. Reading the archive

A `raw` object is the original lines, so read it with whatever you would use on the logs themselves:

```bash
aws s3 cp s3://acme-dns-archive/dns/year=2026/…/logs_01a06bbb-….txt.gz - | gunzip -c
```

An `otlp` object needs the envelope unwrapping first. This works whatever the lines are, because the
body is carried as a string — here, each record as its cluster and its line:

```bash
aws s3 cp s3://acme-dns-archive/dns/year=2026/…/logs_01a06bbb-….json.gz - \
  | gunzip -c \
  | jq -r '.resourceLogs[].scopeLogs[].logRecords[]
           | [(.attributes[] | select(.key=="cluster_id") | .value.stringValue), .body.stringValue]
           | @tsv'
```

Where the lines are themselves JSON — audit events, for instance — put the body through `fromjson`
to reach its fields:

```bash
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
kubectl -n my-namespace delete logexport dns
```

**Objects already written stay in your bucket.** Nothing on the Giant Swarm side ever deletes them,
so the archive is yours to keep and yours to expire — set a lifecycle policy on the bucket if you
want the objects to be deleted.
