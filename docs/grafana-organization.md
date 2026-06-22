# GrafanaOrganization

The `GrafanaOrganization` CRD lets you declare a Grafana organization as a Kubernetes resource. The operator creates and manages the organization in Grafana, configures its datasources, and sets up SSO role mappings.

## How it works

When a `GrafanaOrganization` is created:
1. The operator creates the corresponding organization in Grafana
2. Datasources are configured for each tenant associated with the organization
3. SSO role mappings are applied so users are assigned the correct Grafana role based on their identity provider group membership
4. The organization's `orgID` is written back to `.status.orgID`

## CRD Reference

### Spec

| Field | Required | Description |
|---|---|---|
| `spec.displayName` | yes | Name shown in the Grafana UI. Must be unique across all organizations. |
| `spec.rbac` | yes | SSO role mappings (see below) |
| `spec.tenants` | yes | List of tenants associated with this org (minimum 1) |

### `spec.rbac`

Maps identity provider groups to Grafana roles. The operator reads the `org_attribute_path` configured on the Grafana instance to determine which attribute to match against.

| Field | Required | Description |
|---|---|---|
| `rbac.admins` | yes | Groups granted Admin role (full org access) |
| `rbac.editors` | no | Groups granted Editor role |
| `rbac.viewers` | no | Groups granted Viewer role |

### `spec.tenants`

Each tenant entry grants the organization access to data for that tenant in Mimir/Loki/Tempo.

| Field | Required | Description |
|---|---|---|
| `name` | yes | Tenant ID — must match `^[a-zA-Z_][a-zA-Z0-9_]{0,149}$`. The value `__mimir_cluster` is forbidden. |
| `types` | no | Access types: `data` (read metrics/logs) and/or `alerting` (manage rules/alerts). Defaults to `["data"]`. |

### Status

| Field | Description |
|---|---|
| `status.orgID` | The Grafana organization ID assigned by Grafana |
| `status.dataSources` | List of datasources provisioned for this organization |

## Example

```yaml
apiVersion: observability.giantswarm.io/v1alpha2
kind: GrafanaOrganization
metadata:
  name: my-team
spec:
  displayName: My Team
  rbac:
    admins:
      - my-team-admins
    editors:
      - my-team-engineers
    viewers:
      - my-team-readonly
  tenants:
    - name: my_team
      types:
        - data
        - alerting
    - name: my_team_staging
      types:
        - data
```

## Notes

- `GrafanaOrganization` is cluster-scoped (no namespace)
- Use `v1alpha2` — it is the current and storage version. `v1alpha1` is accepted but deprecated; the conversion webhook handles it automatically.
- Deleting a `GrafanaOrganization` removes the finalizer and cleans up the Grafana organization
