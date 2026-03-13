### What this PR does / why we need it


### Checklist

**Every PR:**
- [ ] `CHANGELOG.md` updated under `[Unreleased]` (skip only for pure refactors with no user-visible effect)
- [ ] Docs updated if the change affects operator behaviour, feature flags, CRD fields, or configuration

**Go controllers:**
- [ ] Reconcile loop delegates to `pkg/`, no direct API calls in controller
- [ ] Errors wrapped with `fmt.Errorf("context: %w", err)`, not swallowed
- [ ] All independent tasks use `errors.Join()` pattern
- [ ] Finalizer added first, removed last
- [ ] New `pkg/` packages have tests (testify table-driven or Ginkgo)
- [ ] Feature flags respected: check both install-level `Enabled` AND per-cluster label
- [ ] RBAC annotations updated if new resources touched (`//+kubebuilder:rbac:...`)

**CRD changes:**
- [ ] Conversion webhook updated for both directions if v1alpha1/v1alpha2 fields change
- [ ] Webhook validation updated for new constraints
- [ ] `zz_generated.deepcopy.go` regenerated (`make generate`)

**Cluster monitoring:**
- [ ] Golden files updated if Alloy config templates change (`make generate-golden-files`)

**Alertmanager:**
- [ ] Golden files updated if Alertmanager config template changes
- [ ] BATS tests cover new route scenario
- [ ] `make tests-alertmanager-routes` passes locally

**Helm:**
- [ ] `values.yaml` updated with the new value (with sensible default)
- [ ] `values.schema.json` updated to match (type, description, required if appropriate)
- [ ] Resource requests/limits set on all containers
- [ ] Image tag not hardcoded
- [ ] Alertmanager template changes validated with `make validate-alertmanager-config`

**Operations:**
- [ ] Breaking changes must be explicitly documented, with a deployment plan
