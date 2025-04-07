# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.23.2] - 2025-04-07

### Fixed

- Fix alloy-rules templating by quoting the tenant label.

## [0.23.1] - 2025-04-07

### Fixed

- Fix `alloy-rules` app version flag rename forgotten after review.

## [0.23.0] - 2025-04-07

### Added

- Add multi-tenancy support to alerting and recording rules loading by setting up the alloy-rules config.
- Add validation script using amtool to validate alertmanager config works.

### Fixed

- Make sure we shard alloy-metrics based on all metrics for all tenants and not for the giantswarm tenant only.

## [0.22.1] - 2025-03-25

### Fixed

- Fix alloy-metrics sharding after we renamed `prometheus.remote_write.default` to `prometheus.remote_write.giantswarm` in alloy config.

## [0.22.0] - 2025-03-24

### Added

- Add multi-tenancy support to alertmanager config loading.

### Changed

- updated run-local.sh to port-forward mimir alertmanager

## [0.21.1] - 2025-03-24

### Fixed

- Set default resources for alloy-metrics only when VPA is enabled.

## [0.21.0] - 2025-03-24

### Added

- Add multi-tenancy support to alloy remote write by creating a custom remote-write section per tenant defined in Grafana Organization CRs.
- Add pod and service monitor discovery of both the old giantswarm team label and the new tenant label.

### Changed

- Fine-tune alloy-metrics resource usage configuration to avoid causing issues for customer workload and cluster tests.

## [0.20.0] - 2025-03-18

### Changed

- Send `severity: page` alerts for Tenet to Opsgenie instead of Slack

## [0.19.4] - 2025-03-14

### Changed

- Revert caching tuning for configmaps and pods because it was causing api false answers.

## [0.19.3] - 2025-03-13

### Changed

- Stop caching helm secrets in the operator to reduce resource usage.
- Cache only the dashboards configmap in the operator to reduce resource usage.
- Cache only the alertmanager and grafana pods in the operator to reduce resource usage.

### Removed

- Remove cleanup code for Mimir Alertmanager anonymous tenant's configuration.
- Remove cleanup code for Mimir Ruler anonymous tenant's rules.

## [0.19.2] - 2025-03-10

### Changed

- Switch mimir default tenant from `anonymous` to `giantswarm` once again.
- Replace deprecated update datasource by id with update datasource by uid Grafana API call.

## [0.19.1] - 2025-03-06

### Changed

- Revert tenant switch and clean up giantswarm tenant

## [0.19.0] - 2025-03-06

### Added

- Add cleanup for Mimir Alertmanager anonymous tenant's configuration.
- Add cleanup for Mimir Ruler anonymous tenant's rules.

### Fixed

- Fix default read tenant to anonymous to ensure grafana rules pages work until the tenant switch is released.

## [0.18.0] - 2025-03-05

### Changed

- Use smaller dockerfile to reduce build time as ABS already generates the go binary.
- Read metrics from both anonymous and giantswarm tenant at once.
- Refactor hardcoded tenant values to prepare the switch from the anonymous to the giantswarm tenant.
- Switch the alerting component from the anonymous to the giantswarm tenant.
- Add Grafana url when there's no dashboard in the alert notification template.

## [0.17.0] - 2025-02-25

### Changed

- update the notification template to take into account the changes in the alert annotations
  - `runbook_url` is now the full url to the runbook
  - `dashboardUid` is now split between `__dashboardUid__` and `dashboardQueryParams`.

## [0.16.0] - 2025-02-20

### Changed

- update the notification template to take into account the new alert annotations
  - `opsrecipe` => `runbook_url`
  - `dashboard` => `dashboardUid`
- improve alert names in opsgenie by:
  - removing the service name if is is not needed
  - removing the cluster-id if the alert targets the installation

## [0.15.0] - 2025-02-10

### Removed

- Clean up `Shared org` specific code that is not needed anymore since we moved the organization declaration to a custom resource (https://github.com/giantswarm/roadmap/issues/3860).

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

[Unreleased]: https://github.com/giantswarm/observability-operator/compare/v0.23.2...HEAD
[0.23.2]: https://github.com/giantswarm/observability-operator/compare/v0.23.1...v0.23.2
[0.23.1]: https://github.com/giantswarm/observability-operator/compare/v0.23.0...v0.23.1
[0.23.0]: https://github.com/giantswarm/observability-operator/compare/v0.22.1...v0.23.0
[0.22.1]: https://github.com/giantswarm/observability-operator/compare/v0.22.0...v0.22.1
[0.22.0]: https://github.com/giantswarm/observability-operator/compare/v0.21.1...v0.22.0
[0.21.1]: https://github.com/giantswarm/observability-operator/compare/v0.21.0...v0.21.1
[0.21.0]: https://github.com/giantswarm/observability-operator/compare/v0.20.0...v0.21.0
[0.20.0]: https://github.com/giantswarm/observability-operator/compare/v0.19.4...v0.20.0
[0.19.4]: https://github.com/giantswarm/observability-operator/compare/v0.19.3...v0.19.4
[0.19.3]: https://github.com/giantswarm/observability-operator/compare/v0.19.2...v0.19.3
[0.19.2]: https://github.com/giantswarm/observability-operator/compare/v0.19.1...v0.19.2
[0.19.1]: https://github.com/giantswarm/observability-operator/compare/v0.19.0...v0.19.1
[0.19.0]: https://github.com/giantswarm/observability-operator/compare/v0.18.0...v0.19.0
[0.18.0]: https://github.com/giantswarm/observability-operator/compare/v0.17.0...v0.18.0
[0.17.0]: https://github.com/giantswarm/observability-operator/compare/v0.16.0...v0.17.0
[0.16.0]: https://github.com/giantswarm/observability-operator/compare/v0.15.0...v0.16.0
[0.15.0]: https://github.com/giantswarm/observability-operator/compare/v0.14.0...v0.15.0
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
