# Dashboard Controllers Architecture

Two controllers cooperate to provision Grafana dashboards from labeled Kubernetes `ConfigMaps`:

| Controller | Name | Responsibility |
|---|---|---|
| `DashboardReconciler` | `dashboard` | Create, update, and delete dashboards in Grafana, and ensure their folder hierarchy exists. |
| `DashboardCleanupReconciler` | `dashboard-cleanup` | Delete operator-managed folders that no longer hold any dashboard, once per organization. |

Both controllers run in the single operator binary and are wired up in `cmd/main.go` behind the `controllers-dashboard-enabled` flag. Split the two responsibilities so that fast, per-ConfigMap dashboard provisioning stays decoupled from slow, per-organization folder cleanup.

## Source of truth

Treat each dashboard `ConfigMap` as the desired state. Select ConfigMaps with the label `app.giantswarm.io/kind: dashboard`, and read the target organization and folder from labels or annotations:

- `observability.giantswarm.io/organization: <org-name>` — required.
- `observability.giantswarm.io/folder: <path>` — optional, slash-separated nested path.

`mapper.DashboardMapper.FromConfigMap` converts a ConfigMap into one `dashboard.Dashboard` domain object per entry in `.data`, attaching the extracted organization and folder path to each.

Derive folder UIDs deterministically from the folder path: `folder.GenerateUID` hashes the full path (SHA-256, first 6 bytes) and prefixes it with `gs-`. Treat any folder whose UID carries the `gs-` prefix as operator-managed. Because the UID depends only on the path, the same path always maps to the same folder, and a path's parent folders share the prefixes of its UID chain.

## Dashboard controller

### Watches

`DashboardReconciler.SetupWithManager` configures two event sources:

- `ConfigMap` objects filtered by the dashboard label selector — the primary trigger.
- `Pod` objects filtered to the Grafana instance, gated by `predicates.GrafanaPodRecreatedPredicate`. When the Grafana pod is recreated, enqueue every dashboard ConfigMap so all dashboards re-provision against the fresh Grafana.

### Reconcile

For each ConfigMap event:

1. Get the ConfigMap. Treat a `NotFound` as nothing to do and return cleanly.
2. Generate a Grafana client and wrap it in a `grafana.Service`.
3. Branch on the deletion timestamp:
   - Zero → `reconcileCreate`.
   - Non-zero → `reconcileDelete`.

**`reconcileCreate`** adds the finalizer `observability.giantswarm.io/grafanadashboard` first, before any mutation, to close the race between provisioning and deletion. Return early after adding the finalizer to let the resulting update re-trigger reconciliation. Then call `ConfigureDashboard` for every dashboard parsed from the ConfigMap.

**`reconcileDelete`** returns immediately if the finalizer is absent. Otherwise call `DeleteDashboard` for every dashboard, then remove the finalizer last, after all deletions succeed.

Both branches route through `processDashboards`, which iterates the parsed dashboards, runs domain validation defensively (in case the validating webhook was bypassed), applies the supplied operation, and joins per-dashboard errors with `errors.Join` so one failure never blocks the others.

Do not perform folder cleanup here. Leave orphaned-folder removal to the cleanup controller so that bursts of dashboard events do not trigger redundant, expensive cleanup passes inline.

### Grafana operations

`grafana.Service` scopes every call to the target organization via `withinOrganization`, which clones the Grafana client with the org ID so concurrent operations across organizations stay isolated.

- `ConfigureDashboard` ensures the folder hierarchy exists (`ensureFolderHierarchy`), strips the `id` field, injects the `managed-by: observability-operator` tag at the schema-appropriate location, and publishes the dashboard with `Overwrite: true`.
- `ensureFolderHierarchy` walks the path segment by segment: create each missing folder with its deterministic UID and parent UID, and rename any folder whose title drifted from the path. Cache the resolved leaf UID per `(orgID, path)` to skip repeated API walks.
- `DeleteDashboard` deletes the dashboard by UID and treats a `NotFound` response as success.

## Dashboard cleanup controller

Key this controller by organization name rather than by ConfigMap. Carry the organization in the reconcile request `Name`, so cleanup runs once per organization regardless of how many ConfigMaps changed.

### Watches and debouncing

`DashboardCleanupReconciler.SetupWithManager` watches dashboard `ConfigMaps` (same label selector) through a custom `handler.Funcs`. On create, update, or delete:

1. Extract the organization from the changed ConfigMap (`organizationRequest`). Drop the event if it is not a ConfigMap or carries no organization.
2. Build a reconcile request whose `Name` is the organization.
3. Enqueue it with `q.AddAfter(req, cleanupDelay)`, where `cleanupDelay` is **one minute**.

The delaying workqueue keeps the earliest scheduled time per key. Collapse a burst of dashboard events for one organization into a single cleanup that runs roughly one minute after the first event of the burst. This debounce absorbs bulk dashboard rollouts and keeps the dashboard controller's throughput high.

Accept that cleanup is eventually consistent: orphaned folders disappear up to about a minute after the last dashboard referencing them is removed.

### Reconcile

For an organization name:

1. Generate a Grafana client and `grafana.Service`.
2. Resolve the organization by name (`FindOrgByName`).
3. Compute the set of folder UIDs still required by listing **all** dashboard ConfigMaps, filtering to those targeting this organization, and collecting the UID of every segment of each folder path (`collectRequiredFolderUIDs`).
4. Delete the orphans via `CleanupOrphanedFoldersForOrg`.

`CleanupOrphanedFoldersForOrg` scopes to the organization, lists all folders, and sorts them deepest-first (by depth derived from the `ParentUID` chain in `folderDepths`). Process leaves before parents so a fully empty nested hierarchy collapses in a single pass. For each folder, delete it only when all three hold:

- The UID is operator-managed (`gs-` prefix).
- It is not in the required-UID set.
- It is empty (`GetFolderDescendantCounts` reports zero descendants).

Skip non-empty orphans with an info log. Collect per-folder errors with `errors.Join`.

## Failure isolation

Run dashboard provisioning and folder cleanup as independent reconcile loops with independent error handling. A cleanup failure never fails dashboard reconciliation, and a single dashboard or folder error never aborts processing of the rest.
