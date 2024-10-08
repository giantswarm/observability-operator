# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Add org CRD in helm chart.

## [0.6.1] - 2024-10-08

### Fixed

- Fix CI jobs generating new releases

## [0.6.0] - 2024-09-24

### Added

- Add manual e2e testing procedure and script.

## [0.5.0] - 2024-09-17

### Changed

- Require observability-bundle >= 1.6.2 for Alloy monitoring agent support, this is due to incorrect alloyMetrics catalog in observability-bundle

### Fixed

- Fix invalid Alloy config due to missing comma on external labels

## [0.4.1] - 2024-09-17

### Fixed

- Disable logger development mode to avoid panicking, use zap as logger
- Fix CircleCI release pipeline

## [0.4.0] - 2024-08-20

### Added

- Add tests with ats in ci pipeline.
- Add helm chart templating test in ci pipeline.
- Add support for Alloy to be used as monitoring agent in-place of Prometheus Agent. This is configurable via the `--monitoring-agent` flag.
- Add Alloy service to manage Alloy monitoring agent configuration
- Add Alloy configuration templates

### Changed

- Move GetClusterShardingStrategy to common/monitoring package
- Add query argument to QueryTSDBHeadSeries
- Removed lll golangci linter

## [0.3.1] - 2024-07-22

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

[Unreleased]: https://github.com/giantswarm/observability-operator/compare/v0.6.1...HEAD
[0.6.1]: https://github.com/giantswarm/observability-operator/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/giantswarm/observability-operator/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/giantswarm/observability-operator/compare/v0.4.1...v0.5.0
[0.4.1]: https://github.com/giantswarm/observability-operator/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/giantswarm/observability-operator/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/giantswarm/observability-operator/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/giantswarm/observability-operator/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/giantswarm/observability-operator/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/giantswarm/observability-operator/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/giantswarm/observability-operator/compare/v0.0.4...v0.1.0
[0.0.4]: https://github.com/giantswarm/observability-operator/compare/v0.0.3...v0.0.4
[0.0.3]: https://github.com/giantswarm/observability-operator/compare/v0.0.2...v0.0.3
[0.0.2]: https://github.com/giantswarm/observability-operator/compare/v0.0.1...v0.0.2
[0.0.1]: https://github.com/giantswarm/observability-operator/releases/tag/v0.0.1
