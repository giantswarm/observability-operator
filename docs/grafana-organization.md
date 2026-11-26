# GrafanaOrganization

The `GrafanaOrganization` CRD lets you declare a Grafana organization as a Kubernetes resource. The operator creates and manages the organization in Grafana, configures its datasources, and sets up SSO role mappings.

## How it works

When a `GrafanaOrganization` is created:
1. The operator creates the corresponding organization in Grafana
2. Datasources are configured for each tenant associated with the organization
3. SSO role mappings are applied so users are assigned the correct Grafana role based on their identity provider group membership (unless [SSO organization mapping](#sso-organization-mapping) is disabled)
4. The organization's `orgID` is written back to `.status.orgID`

## CRD Reference

### Spec

| Field | Required | Description |
|---|---|---|
| `spec.displayName` | yes | Name shown in the Grafana UI. Must be unique across all organizations. |
| `spec.rbac` | yes | SSO role mappings (see below). Ignored when SSO organization mapping is disabled. |
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

## SSO organization mapping

When enabled, the operator writes the `org_mapping` setting of Grafana's `generic_oauth` SSO provider from the `spec.rbac` of all `GrafanaOrganization` resources. The mapping lists only the organizations that exist in Grafana, those with a `status.orgID`: with strict role mapping, Grafana drops the whole mapping when one organization name does not exist.

It is controlled by the `grafana.ssoOrgMapping.enabled` Helm value, which defaults to `false`, and the `--grafana-sso-org-mapping-enabled` flag. Keep it off when Grafana has no generic OAuth provider configured. When it is off:

- the operator neither reads nor writes Grafana's SSO provider settings
- organizations and their datasources are still reconciled as usual
- `spec.rbac` is still required by the API, but its contents are ignored

## JWT organization mapping

The operator writes the organization mapping from the `spec.rbac` of all `GrafanaOrganization` resources to the `orgMapping` key of the `grafana-mcp-org-mapping` Secret in the `monitoring` namespace. The `grafana-mcp` Grafana instance reads it as `GF_AUTH_JWT_ORG_MAPPING`. The mapping has the same entries as the SSO organization mapping.

It is controlled by the `grafana.jwtOrgMappingSecret.enabled` Helm value, which defaults to `true`, and the `--grafana-jwt-org-mapping-secret-enabled` flag. Turn it off when the installation runs no `grafana-mcp`.

Grafana reads `GF_AUTH_JWT_ORG_MAPPING` only at start-up. After it writes the Secret, the operator sets the `observability.giantswarm.io/org-mapping-checksum` annotation on the pod template of the `grafana-mcp` Deployment in the `monitoring` namespace to the sha256 of the mapping:

- a changed mapping rolls the `grafana-mcp` pods
- an unchanged mapping leaves the Deployment as it is
- the operator skips the annotation when the Deployment does not exist

The operator sends a merge patch without reading the Deployment first. The chart grants it only `patch` on the `grafana-mcp` Deployment, with a Role and a RoleBinding in the `monitoring` namespace.

Known limitations:

- The mapping uses `spec.displayName`. After a `displayName` change, the reconcile of another `GrafanaOrganization` can write the new name before Grafana has the renamed organization, or while Grafana rejects the rename. With strict role mapping, Grafana then drops the whole mapping and every user loses their organization roles until the rename succeeds. Both the SSO and the JWT organization mappings are affected.
- A new organization can join the mapping one reconcile late, because the operator lists `GrafanaOrganization` resources from its cache.

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
- Deleting a `GrafanaOrganization` removes the finalizer and cleans up the Grafana organization. If the Grafana admin Secret no longer exists, for example because Grafana was uninstalled along with it, the operator removes the finalizer without contacting Grafana.
