# LogExport selectors

A `LogExport` copies selected log lines to a destination outside the observability platform. `spec.selector` is the LogQL expression that picks them:

```yaml
apiVersion: observability.giantswarm.io/v1alpha1
kind: LogExport
metadata:
  name: audit-logs-to-s3
spec:
  selector: '{cluster_id="wc01", scrape_job="audit-logs"} |= "delete" | json | verb!="get"'
  destination:
    type: s3
    s3:
      bucket: acme-audit-archive
      region: eu-central-1
```

The operator translates that selector into `loki.process` stages for the exporting agent, so the accepted subset is narrower than what Loki's query API takes: a selector has to be *renderable*, not merely valid.

A validating webhook checks it when you apply the CR, so a selector outside the subset fails at `kubectl apply` rather than going quiet in the exporter:

```
The LogExport "audit-logs-to-s3" is invalid: spec.selector: Invalid value:
"{scrape_job=\"audit-logs\"}[5m]": LOGQL001: time ranges are not supported: an
export forwards log lines as they arrive, so there is no past window to select
from; supported is a stream selector (e.g. {scrape_job="audit-logs"}), ...
```

Look the `LOGQLnnn` code up in the [error reference](#error-reference) for the reason and the way around it.

## The supported subset

| Part | Supported | Example |
|---|---|---|
| Stream selector | required, and must name a stream | `{scrape_job="audit-logs"}` |
| Line filters | `\|=`, `!=`, `\|~`, `!~`, chained freely | `\|= "delete" !~ "healthz"` |
| Parser | `\| json` only, with no parameters | `\| json` |
| Label filters | string comparisons `=`, `!=`, `=~`, `!~` on a single label | `\| verb="delete"` |

A label filter before the parser matches a stream label; after `| json` it matches a field of the log line, which the renderer extracts and promotes to a label first.

```
{cluster_id="wc01", scrape_job="audit-logs"} |= "delete" | json | verb!="get"
```

## Why the subset is narrow

Selection is rendered as a **drop of the opposite** of what you wrote, never as a keep. Sequential drops then compose as an AND of keeps.

The selector above becomes:

```
drop     {cluster_id!="wc01"}
drop     {scrape_job!="audit-logs"}
drop     {cluster_id="wc01", scrape_job="audit-logs"} != "delete"
extract  verb                                  # promote the JSON field to a label
drop     {verb="get"}
```

Two consequences follow, and between them they explain most of this page:

- **Every part of a selector needs an opposite the renderer can spell.** A line filter is not a valid selector on its own, so its drop repeats the stream selector; a label filter needs its field extracted before it can be matched on; and a comparison like `| duration > 10s` has no stream-selector spelling once inverted.
- **A part with no opposite is rejected, not approximated.** An approximation either leaks log lines out of the installation or silently discards lines you asked for, and neither is visible from the CR. Rejecting at admission is the only failure mode you can see.

Where a line cannot be evaluated at all — a missing field, an empty label — it is not exported.

## Error reference

Codes are stable. They are never renumbered and never reused, so they are safe to quote in a ticket.

### LOGQL001 — time ranges are not supported

```
{scrape_job="audit-logs"}[5m]
```

An export forwards log lines as they arrive, so there is no past window to select from.

**Instead:** drop the range. `{scrape_job="audit-logs"}`.

### LOGQL002 — not a valid LogQL expression

The expression does not parse, or names no stream. Loki requires at least one matcher that cannot match empty, so `{}`, `{job=~".*"}` and `{job!="x"}` are all rejected — a negative matcher also matches streams that lack the label entirely, which makes it a filter rather than a matcher.

**Instead:** name a stream. `{scrape_job=~".+"}` clears the bar, and how much you narrow beyond that is your call.

### LOGQL003 — returns a value rather than log lines

```
1
vector(0)
```

The expression is a constant, not a stream of log lines.

**Instead:** start from a stream selector.

### LOGQL004 — aggregations are not supported

```
sum by (verb) (rate({scrape_job="audit-logs"}[5m]))
count_over_time({scrape_job="audit-logs"}[1h])
```

An export is a continuous tee, not a query — there is nothing to aggregate over.

**Instead:** export the lines and aggregate them at the destination.

### LOGQL005 — not a log selector

The expression parsed into something that is neither a log selector nor an aggregation. No known input reaches this; if you see it, please report it with the selector.

### LOGQL006 — not a usable log selector

```
{scrape_job="audit-logs"} |~ "("
```

The expression parses, but its regexps do not compile. This is checked at admission because a selector that fails to build reaches Alloy and stops every export on the installation, not just yours.

**Instead:** fix the regexp — here, escape the parenthesis as `\(`.

### LOGQL007 — parser is not supported

```
{scrape_job="audit-logs"} | unpack
{scrape_job="audit-logs"} | pattern "<_> <msg>"
{scrape_job="audit-logs"} | regexp "(?P<verb>\\w+)"
```

Only `| json` is supported. The renderer extracts fields with `stage.json`, which has no equivalent for the other parsers.

**Instead:** if your lines are JSON, use `| json`. If they are not, filter on the raw line with a line filter.

### LOGQL008 — label filter is not supported

Only a string comparison (`=`, `!=`, `=~`, `!~`) on a single label has an opposite the renderer can drop. Numeric, duration, bytes, `ip()` and boolean combinations do not.

| Rejected | Instead |
|---|---|
| `\| status_code >= 400` | `\| status_code=~"4..\|5.."` — compare as a string |
| `\| duration > 10s` | no equivalent; filter at the destination |
| `\| remote_addr = ip("1.2.3.4")` | `\| remote_addr="1.2.3.4"`, or `=~` for a range |
| `\| verb="delete" and user="x"` | `\| verb="delete" \| user="x"` — chained filters already AND |
| `\| verb="delete" or verb="create"` | `\| verb=~"delete\|create"` |

### LOGQL009 — stage is not supported

```
{scrape_job="audit-logs"} | logfmt
{scrape_job="audit-logs"} | line_format "{{ .verb }}"
{scrape_job="audit-logs"} | json verb="fields.verb"
```

Covers everything the renderer has no stage for: `logfmt`, `line_format`, `label_format`, `drop`, `keep`, `decolorize`, and `| json` with parameters.

An export forwards lines unchanged, so the stages that reshape a line have nothing to do here. For a parameterised `| json`, use a plain `| json` and a label filter on the field: `| json | verb="delete"`.

### LOGQL010 — line filter uses `or`

```
{scrape_job="audit-logs"} |= "delete" or "create"
```

The exporter does not support `or` on a positive filter (`|=`, `|~`).

**Instead:** use one regexp. `|~ "delete|create"`.

`or` on a negative filter (`!= "a" or "b"`) is accepted — Loki has already flattened it into `!= "a" != "b"` by the time the operator sees it.

### LOGQL011 — line filter uses `ip()`

```
{scrape_job="audit-logs"} |= ip("1.2.3.4")
```

The exporter does not support `ip()` line filters, in any of the four operators.

**Instead:** match the address as text. `|= "1.2.3.4"`, or a regexp such as `|~ "10\\.0\\.0\\..+"` for a range.

### LOGQL012 — label is reserved for Loki internals

```
{__name__="x", job="y"}
{scrape_job="audit-logs"} | json | __error__=""
```

Labels starting with `__` are produced by Loki's query-time parsers. They do not exist in the exporting agent's pipeline, so a filter on them cannot be evaluated — and would silently export more than you asked for.

**Instead:** filter on a label or field the agent actually sees.

## Widening the subset

The subset is bounded by the renderer, not by taste. Anything admission accepts has to be renderable, so widening the accepted set means teaching the renderer the new form in the same change — see `internal/webhook/validation/logql.go` and `pkg/agent/logexporter/selector.go`.
