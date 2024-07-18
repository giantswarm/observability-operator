# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- Fix some reconcile errors (https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/reconcile#Reconciler).

## [0.3.0] - 2024-07-16

### Changed

- Delete monitoring resources if monitoring is disabled at the installation or cluster level using the giantswarm.io/monitoring label.

## [0.2.0] - 2024-06-25

### Added

- Add per cluster and installation overridable sharding strategy support for mimir-backed installations.

### Fixed

- Fix an issue where remote-write secret was not being created when head series query fails.

## [0.1.1] - 2024-06-14

### Fixed

- Fix reconciliation errors when adding or removing the finalizer on the Cluster CR.

## [0.1.0] - 2024-06-06

### Added

- Add support for mimir in remoteWrite secret creation.
- Add mimir ingress secret for basic auth creation.

## [0.0.4] - 2024-05-28

### Changed

- Do nothing if mimir is disabled to avoid deleting prometheus-meta-operator managed resources.

### Fixed

- Fix mimir heartbeat priority.

### Removed

- Finalizer on operator managed resources (configmap and secrets) as no other operator is touching them.

## [0.0.3] - 2024-05-24

### Changed

- Manage prometheus-agent configs

## [0.0.2] - 2024-04-08

### Fixed

- Fix `CiliumNetworkPolicy` to allow cluster and world access (opsgenie)

## [0.0.1] - 2024-04-08

### Added

- Initialize project and create heartbeat for the installation.

[Unreleased]: https://github.com/giantswarm/observability-operator/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/giantswarm/observability-operator/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/giantswarm/observability-operator/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/giantswarm/observability-operator/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/giantswarm/observability-operator/compare/v0.0.4...v0.1.0
[0.0.4]: https://github.com/giantswarm/observability-operator/compare/v0.0.3...v0.0.4
[0.0.3]: https://github.com/giantswarm/observability-operator/compare/v0.0.2...v0.0.3
[0.0.2]: https://github.com/giantswarm/observability-operator/compare/v0.0.1...v0.0.2
[0.0.1]: https://github.com/giantswarm/observability-operator/releases/tag/v0.0.1
