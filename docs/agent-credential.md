# AgentCredential

The `AgentCredential` CRD lets you declare a single basic-auth credential, scoped to one telemetry agent and one observability backend (metrics → Mimir, logs → Loki, traces → Tempo). The operator mints a Kubernetes `Secret` for the credential and aggregates its htpasswd entry into the per-backend gateway Secret.

`AgentCredential` decouples credential generation from the Cluster lifecycle. Alongside the CRs auto-created by the cluster controller for each CAPI cluster, you can create your own CRs for agents that live outside the managed-cluster surface (for example: a Prometheus running off-cluster, or an OTel collector on a dev laptop).

## How it works

When an `AgentCredential` is created:

1. The controller renders a Secret (same namespace as the CR) with keys `username`, `password`, and `htpasswd`. The username is `<namespace>/<name>` of the CR. The password is generated once on create and preserved across reconciles.
2. It aggregates the htpasswd entries of every AgentCredential matching the same `spec.backend` into the per-backend HTTPRoute gateway Secret (key `.htpasswd`).
3. On deletion, the aggregator rewrites the gateway Secret without the entry before the finalizer is removed.

## Enabling / disabling the controller

The controller is toggled via the `--controllers-agent-credential-enabled` flag (Helm value `operator.controllers.agentCredential.enabled`, default `true`).

The cluster controller depends on the agent-credential controller — it declares AgentCredential CRs per cluster and reads the rendered Secrets through the Alloy collectors. If `controllers.agentCredential.enabled=false`, the operator implicitly disables the cluster controller too and logs a warning at startup.

## CRD Reference

### Spec

| Field | Required | Description |
|---|---|---|
| `spec.backend` | yes | One of `metrics`, `logs`, `traces`. Determines which gateway's htpasswd Secret this credential is aggregated into. |
| `spec.agentName` | no | **Deprecated.** No longer the username. For one release the gateway also accepts it as a legacy username, with the same password, so agents can switch over; a legacy name shared by several credentials of a backend is not accepted. It will be removed in a future release. |
| `spec.secretName` | no | Name of the rendered Secret. Defaults to `metadata.name`. |

### Username

The basic-auth username is always `<namespace>/<name>` of the AgentCredential, for example `monitoring/my-external-collector`. Kubernetes guarantees that namespace/name is unique, so no two credentials can share a username. Read it from the `username` key of the rendered Secret rather than hard-coding it.

When the operator is upgraded from a release that used `spec.agentName` as the username, the next reconcile rewrites the Secret's `username` and `htpasswd` keys with the new username and keeps the existing password.

### Status

| Field | Description |
|---|---|
| `status.secretRef` | Reference to the rendered Secret in the same namespace. |
| `status.conditions` | `Ready` (secret rendered) and `GatewaySynced` (htpasswd aggregated). |

## Password rotation

Passwords are generated once on first reconcile and preserved across subsequent reconciles so existing agents keep working. To rotate a password, delete the rendered `Secret` (the CR keeps its name, status, and owner references). The controller regenerates the `Secret` on the next reconcile and re-aggregates the gateway htpasswd.

## Immutability

The validating webhook rejects updates to `spec.backend` or `spec.secretName`, because changing them strands the existing Secret. To change either field, delete the CR and create a new one.

## Cluster-backed AgentCredentials

For every CAPI Cluster with an enabled observability backend, the cluster controller creates one AgentCredential per backend:

- CR name: `<cluster-name>-observability-<backend>` (namespaced to the Cluster's namespace)
- `spec.backend` matches the backend
- The username is therefore `<cluster-namespace>/<cluster-name>-observability-<backend>`
- `spec.secretName` is `<cluster-name>-observability-<backend>-auth` — same name as the pre-CRD legacy Secret, so Alloy collectors pick up the credential without changes.
- An owner reference to the Cluster cascades deletion when the Cluster is removed.

### Upgrading Giant Swarm installations

Releases before the username was derived from namespace/name used the Cluster name as the username. After the upgrade, per-cluster usernames become `<org-namespace>/<cluster>-observability-<backend>`; passwords are unchanged. Agents don't see the new username at once. The Cluster controller re-renders a cluster's agent configuration as soon as one of its AgentCredential Secrets changes, and the app platform then rolls the configuration out to the Alloy agents. During that window the gateway accepts both the old username (`spec.agentName`) and the new one with the same password, so agents keep authenticating. The legacy username is dropped when `spec.agentName` is removed.

Rolling back to a release before this change after agents have switched reintroduces `401`s, because older releases only accept the old username: roll forward instead, or reconcile every Cluster after the rollback so the agents get the old username back.

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
```

Once reconciled, the Secret `monitoring/my-external-collector` contains the basic-auth material and the Mimir gateway accepts requests authenticated as `monitoring/my-external-collector`.
