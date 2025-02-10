# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.14.0] - 2025-02-10

### Changed

- Configure Alloy-metrics `sample_age_config` so that alloy does not indefinitely retry to send old and rejected samples.

## [0.13.3] - 2025-02-10

### Changed

- Updated the notFound method to match error using runtime.APIError type and the status code
- Update Grafana pod predicate to not trigger on pod deletion
- Ensure organization is created before proceeding with datasources and sso settings
- Remove error handling when the organization name is already taken, this is handled by the Grafana API

### Fixed

- Fix `failed to find organization with ID: 0` error when creating a new organization
- Fix `getOrgByIdForbidden` error when creating a new organization
- Fix race condition when switching organization in Grafana client by using WithOrgID method

## [0.13.2] - 2025-02-06

### Added

- Add datasources UID

### Changed

- improved run-local port-forward management
- Fix missing SSO settings in organizations

### Fixed

- fixed loading dashboards when they have an `id` defined

### Removed

- Remove turtle related Alertmanager configuration
- Remove Alertmanager datasource

## [0.13.1] - 2025-01-30

### Removed

- Remove deprecated code that target an older release.

## [0.13.0] - 2025-01-16

### Added

- Add Alertmanager config and templates in Helm chart (#188)

## [0.12.0] - 2025-01-15

### Changed

- Rename datasources to get rid of the olly-op part.

## [0.11.0] - 2025-01-10

### Added

- command line args to configure mimir and grafana URLs
- Support for loading dashboards in organizations

## [0.10.2] - 2024-12-17

### Added

- Add Alertmanager controller

### Changed

- Change SSO settings configuration to use the Grafana admin API instead of app user-values.

## [0.10.1] - 2024-12-12

### Fixed

- Fix grafana organization reordering.

## [0.10.0] - 2024-12-10

### Added

- Add Mimir Alertmanager datasource.
- Add tenant ids field to the grafana organization CR to be able to support multiple tenants into one organization.

### Changed

- Removed organization OwnerReference on grafana-user-values configmap, this fixes an issue where the configmap is removed when the last organization is deleted which prevent Grafana from starting.

### Fixed

- Fix grafana organization deletion

## [0.9.1] - 2024-11-21

### Fixed

- Fix exclusion check for mimir datasource to use the datasource type instead of the name.

## [0.9.0] - 2024-11-20

### Added

- Add Grafana Organization creation logic in reconciler.
- Add creation and update of Grafana organizations.
- Add configuration of the Grafana org_mapping via user-values.

### Fixed

- Disable crd installation from alloy-metrics as this is causing issues with the new v29 releases.
- Fix failing ATS tests by upgrading python testing dependencies and creating necessary secrets.

## [0.8.1] - 2024-10-17

### Fixed

- Fix `flag redefined` error

## [0.8.0] - 2024-10-17

### Added

- Add wal `truncate_frequency` configuration to alloy-metrics with a default set to 15m.
- Add grafanaOrganization CRD in helm chart.

### Changed

- Change default default monitoring agent to Alloy

## [0.7.1] - 2024-10-10

### Fixed

- [Alloy] Fix CiliumNetworkPolicy to allow Alloy to reach out to every pods in the cluster

## [0.7.0] - 2024-10-10

### Changed

- [Alloy] Enable VPA for AlloyMetrics
- Change the PromQL query used to determine the amount of head series when scaling Prometheus Agent and Alloy ([#74](https://github.com/giantswarm/observability-operator/pull/74))

### Fixed

- [Alloy] Fix an issue where monitoring agent is the configured to be the same for all clusters
- Monitoring agents: keep currently configured shards when failing to compute shards

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

[Unreleased]: https://github.com/giantswarm/observability-operator/compare/v0.14.0...HEAD
[0.14.0]: https://github.com/giantswarm/observability-operator/compare/v0.13.3...v0.14.0
[0.13.3]: https://github.com/giantswarm/observability-operator/compare/v0.13.2...v0.13.3
[0.13.2]: https://github.com/giantswarm/observability-operator/compare/v0.13.1...v0.13.2
[0.13.1]: https://github.com/giantswarm/observability-operator/compare/v0.13.0...v0.13.1
[0.13.0]: https://github.com/giantswarm/observability-operator/compare/v0.12.0...v0.13.0
[0.12.0]: https://github.com/giantswarm/observability-operator/compare/v0.11.0...v0.12.0
[0.11.0]: https://github.com/giantswarm/observability-operator/compare/v0.10.2...v0.11.0
[0.10.2]: https://github.com/giantswarm/observability-operator/compare/v0.10.1...v0.10.2
[0.10.1]: https://github.com/giantswarm/observability-operator/compare/v0.10.0...v0.10.1
[0.10.0]: https://github.com/giantswarm/observability-operator/compare/v0.9.1...v0.10.0
[0.9.1]: https://github.com/giantswarm/observability-operator/compare/v0.9.0...v0.9.1
[0.9.0]: https://github.com/giantswarm/observability-operator/compare/v0.8.1...v0.9.0
[0.8.1]: https://github.com/giantswarm/observability-operator/compare/v0.8.0...v0.8.1
[0.8.0]: https://github.com/giantswarm/observability-operator/compare/v0.7.1...v0.8.0
[0.7.1]: https://github.com/giantswarm/observability-operator/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/giantswarm/observability-operator/compare/v0.6.1...v0.7.0
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
