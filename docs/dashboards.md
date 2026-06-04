# Dashboard Provisioning

The operator watches for Kubernetes `ConfigMaps` containing Grafana dashboard JSON and provisions them into the correct Grafana organization.

## How it works

1. Create a `ConfigMap` with the required labels and your dashboard JSON as the data
2. The operator detects the ConfigMap, determines the target organization and folder, and provisions the dashboard via the Grafana API
3. On deletion, the operator removes the dashboard from Grafana and cleans up any empty operator-managed folders

## Required labels

| Label/Annotation | Where | Description |
|---|---|---|
| `app.giantswarm.io/kind: dashboard` | label | Marks the ConfigMap as a dashboard |
| `observability.giantswarm.io/organization: <org-name>` | label or annotation | The `GrafanaOrganization` name to provision into |

## Optional: folder placement

| Label/Annotation | Where | Description |
|---|---|---|
| `observability.giantswarm.io/folder: <path>` | label or annotation | Folder path within the organization. Supports nested hierarchy using `/` as separator (e.g. `platform/networking`). The operator creates the full folder hierarchy if it does not exist. |

## Example — basic

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-dashboard
  namespace: monitoring
  labels:
    app.giantswarm.io/kind: dashboard
    observability.giantswarm.io/organization: my-team
data:
  my-dashboard.json: |
    {
      "title": "My Dashboard",
      "panels": []
    }
```

## Example — with folder

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-dashboard
  namespace: monitoring
  labels:
    app.giantswarm.io/kind: dashboard
  annotations:
    observability.giantswarm.io/organization: my-team
    observability.giantswarm.io/folder: platform/networking
data:
  my-dashboard.json: |
    {
      "title": "Network Overview",
      "panels": []
    }
```

## Dashboard schema

Both Grafana dashboard schemas are supported:

- **Classic flat JSON** — the top-level `uid` field identifies the dashboard.
- **App Platform schema** (`apiVersion: dashboard.grafana.app/v2`) — a Kubernetes-style envelope where the dashboard is identified by `metadata.name` and its body lives under `spec`.

The operator detects the schema automatically and extracts the UID from the appropriate field (`uid` for the flat schema, `metadata.name` for v2). The `managed-by: observability-operator` tag is injected at the correct location for each schema (top-level `tags` for the flat schema, `spec.tags` for v2).

## Notes

- A ConfigMap targets exactly one `GrafanaOrganization`. You can have multiple ConfigMaps per org.
- The folder path must use `/` as a separator. Each segment becomes a nested Grafana folder.
- Operator-managed folders are tracked by UID. If a folder is renamed manually in Grafana, the operator will rename it back to match the path.
- Empty operator-managed folders are deleted when no dashboard references them.
