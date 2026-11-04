# Dashboard Controllers Architecture

Two controllers provision Grafana dashboards from labeled Kubernetes `ConfigMaps`:

| Controller | Name | Responsibility |
|---|---|---|
| `DashboardReconciler` | `dashboard` | Create, update, and delete dashboards in Grafana, and ensure their folder hierarchy exists. |
| `DashboardCleanupReconciler` | `dashboard-cleanup` | Delete operator-managed folders that no longer hold any dashboard, once per organization. |

![Dashboard provisioning and cleanup flow](images/dashboard-controller-architecture.png)

## Source of truth

Each dashboard `ConfigMap` carries the desired state. Both controllers select ConfigMaps with the label `app.giantswarm.io/kind: dashboard` and read the target organization and folder from labels or annotations:

- `observability.giantswarm.io/organization: <org-name>` — required.
- `observability.giantswarm.io/folder: <path>` — optional, slash-separated nested path.

`mapper.DashboardMapper.FromConfigMap` converts a ConfigMap into one `dashboard.Dashboard` domain object per entry in `.data`, attaching the extracted organization and folder path to each.

A ConfigMap holding a dashboard for org `giantswarm` under folder `platform/networking`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: networking-overview
  labels:
    app.giantswarm.io/kind: dashboard
    observability.giantswarm.io/organization: giantswarm
    observability.giantswarm.io/folder: platform/networking
data:
  overview.json: '{"uid": "net-overview", "title": "Overview", ...}'
```

maps to a domain object:

```
Dashboard{uid: net-overview, organization: giantswarm, folderPath: platform/networking}
```

Folder UIDs are deterministic: `folder.GenerateUID` hashes the full path (SHA-256, first 6 bytes) and prefixes it with `gs-`. A folder whose UID carries the `gs-` prefix is operator-managed. The UID depends only on the path, so the same path always maps to the same folder.

```
""                    → ""              (General folder, no UID)
"platform"            → gs-d294fcce0cc8
"platform/networking" → gs-ad98d443b621
```

A nested folder path expands into one folder per segment, each parented to the previous:

```
platform/networking
└─ platform              uid=gs-d294fcce0cc8  parent=""
   └─ networking         uid=gs-ad98d443b621  parent=gs-d294fcce0cc8
```

## Dashboard controller

### Watches

`DashboardReconciler.SetupWithManager` configures two event sources:

- `ConfigMap` objects filtered by the dashboard label selector.
- `Pod` objects filtered to the Grafana instance, gated by `predicates.GrafanaPodRecreatedPredicate`. When the Grafana pod is recreated, every dashboard ConfigMap is enqueued so all dashboards re-provision against the fresh Grafana.

### Reconcile

For each ConfigMap event:

1. Get the ConfigMap. A `NotFound` returns cleanly.
2. Generate a Grafana client and wrap it in a `grafana.Service`.
   If the ConfigMap is being deleted and the Grafana admin Secret is missing, the controller removes the finalizer and returns. The dashboard stays in Grafana.
3. Branch on the deletion timestamp:
   - Zero → `reconcileCreate`.
   - Non-zero → `reconcileDelete`.

**`reconcileCreate`** adds the finalizer `observability.giantswarm.io/grafanadashboard` before any mutation and returns early after adding it. It then calls `ConfigureDashboard` for every dashboard parsed from the ConfigMap.

**`reconcileDelete`** returns immediately if the finalizer is absent. Otherwise it calls `DeleteDashboard` for every dashboard, then removes the finalizer last.

Both branches route through `processDashboards`, which iterates the parsed dashboards, runs domain validation on each, applies the supplied operation, and joins per-dashboard errors with `errors.Join`. Folder cleanup is not performed in this controller.

### Grafana operations

`grafana.Service` scopes every call to the target organization via `withinOrganization`, which clones the Grafana client with the org ID.

- `ConfigureDashboard` ensures the folder hierarchy exists (`ensureFolderHierarchy`), strips the `id` field, injects the `managed-by: observability-operator` tag and the https://github.com/giantswarm/dashboards link at the schema-appropriate location, and publishes the dashboard through the Grafana app-platform API (`/apis/dashboard.grafana.app`). See [Managed dashboards](#managed-dashboards).
- `ensureFolderHierarchy` walks the path segment by segment: it creates each missing folder with its deterministic UID and parent UID, and renames any folder whose title drifted from the path. It caches the resolved leaf UID per `(orgID, path)`.
- `DeleteDashboard` deletes the dashboard by UID through the legacy API (`DELETE /api/dashboards/uid/<uid>`) and treats a `NotFound` response as success.

### Managed dashboards

`publishDashboard` writes a Kubernetes-style `Dashboard` resource:

- Classic schema: `apiVersion: dashboard.grafana.app/v1`, `metadata.name` set to the dashboard UID, `spec` set to the dashboard JSON.
- v2 schema: the input `apiVersion`, `kind` and `spec`. `metadata` keeps only `name`. Server metadata copied from Grafana (`resourceVersion`, `generation`, `creationTimestamp`, `uid`, `labels`, `annotations`, `namespace`) is dropped.

Both schemas carry these annotations:

| Annotation | Value |
|---|---|
| `grafana.app/managedBy` | `kubectl` |
| `grafana.app/managerId` | `observability-operator` |
| `grafana.app/sourcePath` | `configmap <namespace>/<name>` |
| `grafana.app/folder` | Leaf folder UID. Omitted for the General folder. |

The write targets the organization namespace: `default` for org 1, `org-<id>` for every other org. It uses API version `v1` for classic input and `v2` for v2 input. `publishDashboard` gets the dashboard first. A `404` creates it with `POST`. Otherwise, a `PUT` without `resourceVersion` replaces it. The first `PUT` on a dashboard written by an older operator adds the annotations.

Grafana shows the dashboards as managed and blocks Save in the UI. Users edit a dashboard, use "Copy JSON to clipboard" and paste the JSON into the ConfigMap. The copied JSON uses the v2 schema. Each dashboard links to https://github.com/giantswarm/dashboards, the source of truth for the dashboards.

## Dashboard cleanup controller

This controller is keyed by organization name rather than by ConfigMap. The organization is carried in the reconcile request `Name`, so cleanup runs once per organization.

An **orphaned folder** is an operator-managed folder (`gs-` prefix) whose UID matches no segment of any folder path declared by the organization's dashboard ConfigMaps. Orphan status depends only on the ConfigMaps. It does not depend on the folder content or on its parent in Grafana. A folder becomes orphaned when the last ConfigMap declaring its path is deleted, changes its folder, or changes its organization.

### Watches and debouncing

`DashboardCleanupReconciler.SetupWithManager` watches dashboard `ConfigMaps` (same label selector) through a custom `handler.Funcs`. On create, update, or delete:

1. `organizationRequest` extracts the organization from the changed ConfigMap. The event is dropped if the object is not a ConfigMap or carries no organization.
2. A reconcile request is built whose `Name` is the organization.
3. It is enqueued with `q.AddAfter(req, cleanupDelay)`, where `cleanupDelay` is **one minute**.

The delaying workqueue keeps the earliest scheduled time per key. A burst of dashboard events for one organization collapses into a single cleanup that runs roughly one minute after the first event of the burst. Cleanup is eventually consistent: orphaned folders disappear up to about a minute after the last dashboard referencing them is removed.

### Reconcile

For an organization name:

1. Generate a Grafana client and `grafana.Service`.
2. Resolve the organization by name (`FindOrgByName`).
3. Compute the set of folder UIDs still required (`collectRequiredFolderUIDs`): list all dashboard ConfigMaps, filter to those targeting this organization, and collect the UID of every segment of each folder path.
4. Delete the orphans via `CleanupOrphanedFoldersForOrg`.

`CleanupOrphanedFoldersForOrg` scopes to the organization, lists all folders, and sorts them deepest-first by depth derived from the `ParentUID` chain (`folderDepths`), so leaves are processed before parents. For each folder, it deletes the folder only when all three hold:

- The UID is operator-managed (`gs-` prefix).
- It is not in the required-UID set.
- It is empty (`GetFolderDescendantCounts` reports zero descendants).

Grafana reports four descendant counts per folder: subfolders, dashboards, library panels, and alert rules. An orphaned folder is non-empty when any count is above zero. Typical content is a dashboard or subfolder created from the Grafana UI. The controller skips a non-empty orphaned folder with an info log. It does not report an error. Per-folder errors are collected with `errors.Join`.

For org `giantswarm` whose only dashboard now lives in `platform/networking`, the required set is `{gs-d294fcce0cc8, gs-ad98d443b621}`. Given these folders in Grafana:

```
gs-d294fcce0cc8  platform              required           → keep
gs-ad98d443b621  platform/networking   required           → keep
gs-1a2b3c4d5e6f  platform/legacy       orphan, empty      → delete
gs-7f8e9d0c1b2a  platform/storage      orphan, 1 UI dashboard → keep (not empty)
team-a-folder    (manually created)    not gs- prefixed   → keep
```

Processing deepest-first deletes `platform/legacy` before re-evaluating `platform`; `platform` survives because it still holds the required `networking` subtree.

## Failure isolation

Dashboard provisioning and folder cleanup are independent reconcile loops with independent error handling. A cleanup failure does not fail dashboard reconciliation, and a single dashboard or folder error does not abort processing of the rest.
