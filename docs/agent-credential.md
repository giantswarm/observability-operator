# AgentCredential

The `AgentCredential` CRD lets you declare a single basic-auth credential, scoped to one telemetry agent and one observability backend (metrics → Mimir, logs → Loki, traces → Tempo). The operator mints a Kubernetes `Secret` for the credential and aggregates its htpasswd entry into the per-backend gateway Secret.

`AgentCredential` decouples credential generation from the Cluster lifecycle. Alongside the CRs auto-created by the cluster controller for each CAPI cluster, you can create your own CRs for agents that live outside the managed-cluster surface (for example: a Prometheus running off-cluster, or an OTel collector on a dev laptop).

## How it works

When an `AgentCredential` is created and the operator is running in `basicAuth` mode:

1. The controller renders a Secret (same namespace as the CR) of type `kubernetes.io/basic-auth` with keys `username`, `password`, and `htpasswd`. The password is generated once on create and preserved across reconciles.
2. It aggregates the htpasswd entries of every AgentCredential matching the same `spec.backend` into the per-backend gateway Secrets (Ingress and HTTPRoute).
3. On deletion, the aggregator rewrites the gateway Secret without the entry before the finalizer is removed.

## Auth mode

The operator has a single, installation-wide flag — `--auth-mode`, wired via the Helm value `auth.mode`:

- `basicAuth` (default): the AgentCredential controller runs and reconciles CRs as described above.
- `none`: the AgentCredential controller is not registered, and the cluster controller does not create AgentCredential CRs. Use this when authentication is enforced at the gateway layer (for example, workload identity). Any existing CRs and Secrets are left alone; operators are expected to clean them up manually to avoid accidental deletion when flipping the mode.

## CRD Reference

### Spec

| Field | Required | Description |
|---|---|---|
| `spec.backend` | yes | One of `metrics`, `logs`, `traces`. Determines which gateway's htpasswd Secret this credential is aggregated into. |
| `spec.agentName` | yes | Basic-auth username. Alphanumeric plus `-` / `_`, max 253 characters. RFC 7617 forbids `:` in basic-auth usernames. |
| `spec.secretName` | no | Name of the rendered Secret. Defaults to `metadata.name`. |

### Status

| Field | Description |
|---|---|
| `status.secretRef` | Reference to the rendered Secret in the same namespace. |
| `status.conditions` | `Ready` (secret rendered) and `GatewaySynced` (htpasswd aggregated). |

## Uniqueness and immutability

The validating webhook rejects:

- Two AgentCredentials with the same `(backend, agentName)` pair — they would produce conflicting htpasswd entries in the aggregated gateway Secret.
- Updates to `spec.backend`, `spec.agentName`, or `spec.secretName` — changing these strands the existing Secret.

To change any of these fields, delete the CR and create a new one.

## Cluster-backed AgentCredentials

For every CAPI Cluster with an enabled observability backend, the cluster controller creates one AgentCredential per backend:

- CR name: `<cluster-name>-observability-<backend>` (namespaced to the Cluster's namespace)
- `spec.backend` matches the backend
- `spec.agentName` is the Cluster name
- `spec.secretName` is `<cluster-name>-observability-<backend>-auth` — same name as the pre-CRD legacy Secret, so Alloy collectors pick up the credential without changes.
- An owner reference to the Cluster cascades deletion when the Cluster is removed.

## Self-service example

Mint a credential for an external OTel collector that writes metrics to Mimir:

```yaml
apiVersion: observability.giantswarm.io/v1alpha1
kind: AgentCredential
metadata:
  name: my-external-collector
  namespace: monitoring
spec:
  backend: metrics
  agentName: my-external-collector
```

Once reconciled, the Secret `monitoring/my-external-collector` contains the basic-auth material and the Mimir gateway accepts requests authenticated as `my-external-collector`.
