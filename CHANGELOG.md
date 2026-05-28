# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Add `kubernetes.io/arch=arm64:NoSchedule` toleration to the generated alloy-logs daemonset values so the DaemonSet schedules on ARM worker nodes.

### Changed

- Bump `github.com/grafana/grafana-openapi-client-go` to `v0.0.0-20260430175825-547a3b5a00a5`. The new release makes `WithOrgID` non-mutating (returns a clone) and stops mutating the package-level `http.DefaultTransport` — the latter was a real data race in the previous transport setup.
- `GrafanaClient.WithOrgID` now returns a fresh client; `OrgID()` was removed from the interface. Service helpers (`ConfigureDashboard`, `DeleteDashboard`, `ConfigureDatasource`, `CleanupOrphanedFoldersForOrg`) thread the per-org client through `withinOrganization` instead of mutating shared state with a save/restore dance.
- Upgrade Cluster API import from v1beta1 to v1beta2 (sigs.k8s.io/cluster-api/api/core/v1beta2).

## [0.68.0] - 2026-04-27
