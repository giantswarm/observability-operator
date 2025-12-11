# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Add enabling/disabling of the logging and events/tracing alloys towards the removal of the logging-operator.
- Use authManager for logs and traces. This is creating the secrets per cluster as well as the ingress and http route secrets but those are not used yet.

## [0.51.2] - 2025-12-08

### Fixed

- Fix GrafanaOrganization v1alpha2 validation webhook service name to match Helm deployment.

## [0.51.1] - 2025-12-08

### Fixed

- Fix CRD conversion webhook service name to match Helm deployment.

## [0.51.0] - 2025-12-08

### Added

- Add v1alpha2 API with conversion webhooks and enhanced multi-tenant configuration support for GrafanaOrganization CRD with more granular data access types.
- Add new `observability_operator_alertmanager_routes` to count the number of routes per tenant.

### Changed

- Create a new secret for HTTPRoute basic auth for Mimir.
- Refactor Mimir authentication to use per-cluster passwords with centralized secret management.
- Extract authentication logic into reusable auth package, eliminating MimirService wrapper.
- Move password generation into auth package for complete self-containment.
- Consolidate cluster password retrieval into auth package, removing duplicate functionality from monitoring package.
- Use cluster name as user in Alloy secret to authenticate against Mimir.
- Set `app` label on metrics to the value of the ServiceMonitor/PodMonitor target resource `app.kubernetes.io/instance` label

### Removed

- Remove legacy alloy configurations

## [0.50.0] - 2025-11-13

### Removed

- Remove OpsGenie management.

## [0.49.0] - 2025-11-12

### Added

- Add Alertmanager routes unit tests
- Add Alertmanager routes integration tests
  test scenarios: heartbeat alerts, pipeline stable and testing alerts, Slack notifications, and ticket creation.

### Changed

- Refactored Grafana package to use domain organization objects and moved status updates to controller layer.
- Refactor webhook test suite architecture to use shared testutil package and individual test suites per API version

### Removed

- Remove prometheus-agent support as we now fully run alloys.
- Remove all opsgenie heartbeats.

## [0.48.1] - 2025-11-05

### Fixed

- Fix cronitor.io integration in proxy environments.

## [0.48.0] - 2025-11-04

### Added

- Add Cronitor.io integration to replace Opsgenie Heartbeats.

## [0.47.0] - 2025-11-04

### Added

- Implement metrics for grafanaOrganizations monitoring:
  - `observability_operator_grafana_organization_info`: Displays the list of organization and the current status in Grafana (active, pending, error)
  - `observability_operator_grafana_organization_tenant_info`: List of the configured tenants per organization

## [0.46.2] - 2025-10-29

### Fixed

- Fixed queue configuration flags not being applied to Alloy remote write configuration

## [0.46.1] - 2025-10-28

### Fixed

- Fixed pagerduty routing

## [0.46.0] - 2025-10-27

### Added

- Add GitHub webhook receivers for team-based alert routing to create GitHub issues for alerts with severity "ticket"
 
### Fixed

- Fixed pagerdutyToken config

## [0.45.1] - 2025-10-15

### Changed

- Update internal service port for MC alloy instances.

## [0.45.0] - 2025-10-15

### Changed

- Send MC metrics via internal service instead of ingress.

## [0.44.0] - 2025-10-14

### Changed

- Alertmanager / PagerDuty: only send alerts that have to page
- Alertmanager / PagerDuty: team routing with 1 token per team

## [0.43.1] - 2025-10-07

### Fixed

- Fix `ScrapeConfig` support to only be available for observability-bundles 2.2.0 (v32+).

## [0.43.0] - 2025-10-06

### Added

- Add support for PrometheusOperator `ScrapeConfig` CRDs. This requires --stability.level=experimental
- Add logging-enabled flag towards the logging-operator -> observability-operator merger.

## [0.42.0] - 2025-09-17

### Added

- Add Tempo datasource support for distributed tracing
  - Conditional creation based on `--tracing-enabled` flag and Helm values support via `tracing.enabled` configuration (defaults to `false`)
  - Full integration with service maps, traces-to-logs, and traces-to-metrics correlations
  - Connects to `http://tempo-query-frontend.tempo.svc:3200` service
  - Comprehensive test coverage for both enabled and disabled scenarios
- Update Loki datasource for logs-to-traces support

### Changed

- Added a `generateDatasources` to generate all datasource needed for an organization
- Major refactoring of the `ConfigureDefaultDatasources` method into `ConfigureDatasource`
- Remove explicit logic to delete `gs-mimir-old` datasource (now covered)
- Optimize GrafanaOrganization status update to only be performed when necessary

## [0.41.0] - 2025-09-01

### Added

- Add configurable `queue_config` fields for Alloy remote write. All Alloy `queue_config` fields (`batch_send_deadline`, `capacity`, `max_backoff`, `max_samples_per_send`, `max_shards`, `min_backoff`, `min_shards`, `retry_on_http_429`, `sample_age_limit`) are now configurable via helm values and command line flags. When not configured, Alloy defaults are used.

### Fixed

- Fix an issue where organizations were not being deleted in Grafana when the corresponding GrafanaOrganization CR was deleted.
- Fix an issue where SSO settings contained configuration for organizations that no longer existed.

## [0.40.0] - 2025-08-27

### Added

- Send all_pipelines alert label to PagerDuty

## [0.39.0] - 2025-08-27

### Added

- Add Alertmanager PagerDuty heartbeat route

### Changed

- Update PagerDuty notification template, to include relevant information about the alert.
- Upgrade `github.com/grafana/prometheus-alertmanager` dependency after the new mimir release.

### Fixed

- Fixed index out of range error in Alertmanager notification template

## [0.38.0] - 2025-08-25

### Added

- `Mimir Cardinality` datasource for Grafana `Shared Org`
- Add Alertmanager PagerDuty router

## [0.37.0] - 2025-08-13

### Changed

- Update `UpsertOrganization` in /pkg/grafana/grafana.go file so that it can update GrafanaOrganization CRs with a matching Grafana Org's ID given that both share the same name.

## [0.36.0] - 2025-07-18

### Removed

- CRDs are managed via MCB so we need to clean them up from the operator.

## [0.35.0] - 2025-07-10

### Changed

- Alloy-metrics RAM reservations / limits tuning.

## [0.34.0] - 2025-07-02

### Added

- **Dashboard domain validation**: Added `pkg/domain/dashboard/` package with Dashboard type and validation rules (UID format, organization presence, content structure)
- **Dashboard mapper**: Added `internal/mapper/` package for converting ConfigMaps to domain objects

### Changed

- **Dashboard processing**: Refactored controller and Grafana service to use domain objects and mapper pattern for better separation of concerns
- **Dashboard ConfigMap validation webhook**:
  - Added Kubernetes validating webhook to validate dashboard ConfigMaps with `app.giantswarm.io/kind=dashboard` label.
  - Includes comprehensive test coverage, Helm chart integration with `webhook.validatingWebhooks.dashboardConfigMap.enabled` configuration, and kubebuilder scaffolding.
  - Webhook is validating dashboard JSON structure and required fields.
- **Dashboard domain validation**: Added `pkg/domain/dashboard/` package with Dashboard type and validation rules (UID format, organization presence, content structure)
- **Dashboard mapper**: Added `internal/mapper/` package for converting ConfigMaps to domain objects

### Changed

- **Dashboard processing**: Refactored controller and Grafana service to use domain objects and mapper pattern for better separation of concerns
- **Dashboard ConfigMap validation webhook**:
  - Added Kubernetes validating webhook to validate dashboard ConfigMaps with `app.giantswarm.io/kind=dashboard` label.
  - Includes comprehensive test coverage, Helm chart integration with `webhook.validatingWebhooks.dashboardConfigMap.enabled` configuration, and kubebuilder scaffolding.
  - Webhook is validating dashboard JSON structure and required fields.
- **Dashboard domain validation**: Added `pkg/domain/dashboard/` package with Dashboard type and validation rules (UID format, organization presence, content structure)
- **Dashboard mapper**: Added `internal/mapper/` package for converting ConfigMaps to domain objects

### Changed

- **Dashboard processing**: Refactored controller and Grafana service to use domain objects and mapper pattern for better separation of concerns
- **Dashboard ConfigMap validation webhook**:
  - Added Kubernetes validating webhook to validate dashboard ConfigMaps with `app.giantswarm.io/kind=dashboard` label.
  - Includes comprehensive test coverage, Helm chart integration with `webhook.validatingWebhooks.dashboardConfigMap.enabled` configuration, and kubebuilder scaffolding.
  - Webhook is ready for business logic implementation to validate dashboard JSON structure and required fields.
- New `cancel_if_cluster_broken` alertmanager inhibition.

## [0.33.1] - 2025-06-19

### Fixed

- Fixed TenantID validation for Alloy compatibility - was causing alloy to crash with some tenant names. Now follows alloy component naming requirements (https://grafana.com/docs/alloy/latest/get-started/configuration-syntax/syntax/#identifiers), which is more restrictive than previously-used mimir requirements.

## [0.33.0] - 2025-06-16

### Added

- Alertmanager inhibition rule `cancel_if_metrics_broken`
- **Alertmanager version synchronization and dependency management**: Added comprehensive automated tooling to ensure Alertmanager fork version stays in sync with Mimir releases (https://github.com/giantswarm/giantswarm/issues/33621):
  - New script `hack/bin/check-alertmanager-version.sh` that compares the local Alertmanager version with the version used in the latest stable Mimir release
  - GitHub Actions workflow `.github/workflows/check-alertmanager-version.yml` that runs the check on `go.mod` changes and can be triggered manually
  - Updated Renovate configuration to automatically track Mimir releases and ignore release candidates, alpha, and beta versions
  - Added comprehensive comments to `renovate.json5` explaining the Alertmanager version tracking logic
  - Updated `go.mod` comments to reference Renovate automation instead of manual version checking

### Changed

- Comprehensive Helm chart support for webhook configuration:
  - Made all webhook resources conditional (ValidatingWebhookConfiguration, Service, Certificate)
  - Added granular enable/disable controls for individual webhooks (``webhook.validatingWebhooks.alertmanagerConfig.enabled`)
  - Added `ENABLE_WEBHOOKS` environment variable configuration
- Replace the `prometheus/alertmanager` package with Grafana's Mimir fork (`grafana/prometheus-alertmanager`) to ensure configuration compatibility between our validating webhook and Mimir's Alertmanager. This change addresses a compatibility issue where the webhook validation logic used the upstream Prometheus Alertmanager config parser, while Mimir uses a fork with additional/modified configuration options. The replacement ensures 100% compatibility and eliminates the risk of configuration drift between validation and runtime. Uses version `v0.25.1-0.20250305143719-fa9fa7096626` corresponding to Mimir 2.16.0.
- Improved Alertmanager configuration validation script (`hack/bin/validate-alertmanager-config.sh`):
  - Automatically extracts the exact commit hash from `go.mod` replacement directive to ensure perfect consistency with webhook validation
  - Replaced binary building with `go run` for faster execution and simpler maintenance
  - Enhanced logging throughout the script for better debugging and monitoring
  - Now uses the exact same Grafana fork commit as the operator's webhook validation logic
- Enhanced AlertmanagerConfigSecret webhook with improved scope filtering and error handling
- Enhanced TenantID validation in GrafanaOrganization CRD to support full Grafana Mimir specification:
  - Expanded pattern from `^[a-z]*$` to `^[a-zA-Z0-9!._*'()-]+$` to allow alphanumeric characters and special characters (`!`, `-`, `_`, `.`, `*`, `'`, `(`, `)`)
  - Increased maximum length from 63 to 150 characters
  - Added validating webhook to enforce forbidden values (`.`, `..`, `__mimir_cluster`) and prevent duplicate tenants

### Fixed

- Fixed alertmanager configuration key consistency across codebase (standardized on `alertmanager.yaml` instead of mixed `alertmanager.yml`/`alertmanager.yaml`)
- Fixed error message formatting in `ExtractAlertmanagerConfig` function

### Removed

- Remove unnecessary Grafana SSO configuration override for `role_attribute_path` and `org_attribute_path`. Any override should happen in shared-configs instead.

## [0.32.1] - 2025-06-03

### Fixed

- Fix Alloy image templating when the alloy app is running the latest version.

## [0.32.0] - 2025-06-02

### Changed

- Updated Alloy configuration (`pkg/monitoring/alloy/configmap.go` and `pkg/monitoring/alloy/templates/monitoring-config.yaml.template`):
    - Conditionally set `alloy.alloy.image.tag` in `monitoring-config.yaml.template`. The operator now explicitly sets the tag to `1.8.3` if the deployed `alloy-metrics` app version is older than `0.10.0`. For `alloy-metrics` app versions `0.10.0` or newer, the image tag will rely on the Alloy Helm chart's defaults or user-provided values, facilitating easier Alloy image updates via the chart.
    - Adjusted indentation for `AlloyConfig` in `monitoring-config.yaml.template` from `indent 8` to `nindent 8`.
- Improve Mimir Datasource configuration (https://github.com/giantswarm/giantswarm/issues/33470)
  - Enable medium level caching (caching of `/api/v1/label/${name}/values`, `/api/v1/series`, `/api/v1/labels` and `/api/v1/metadata` for 10 minutes)
  - Enable incremental querying (only query new data when refreshing dashboards)

### Removed

- Remove old mimir datasource on all installations.

## [0.31.0] - 2025-05-15

### Changed

- Unify finalizer logic accross controllers
- We decided to disable the alerting tab in the Shared Org to prevent customers from messing with our alerts so we need to open the alert in orgID = 2 (i.e. the Giant Swarm organization).

### Removed

- Remove deprecated slackAPIURL property.

## [0.30.0] - 2025-05-14

### Removed

- Remove crds template as the CRDs are now deployed via management-cluster-bases (https://github.com/giantswarm/management-cluster-bases/pull/232)

### Changed

- Grafana API client is now generated on every requests to support grafana secret changes and allows for better mc-bootstrap testing (https://github.com/giantswarm/giantswarm/issues/32664)
- Updated alertmanager's inhibitions, getting rid of vintage-specifics.
- Clean up old teams and unused inhibitions.

## [0.29.0] - 2025-05-05

### Changed

- Switch alloy-metrics secret from env variables to alloy `remote.kubernetes.secret` component to support secret changes without having to terminate pods.

## [0.28.0] - 2025-04-29

### Fixed

- Fix alertmanager configuration to not drop alerts when stable-testing management cluster's default apps are failing.

### Removed

- Remove alloy-rules deletion code which is no longer needed since the last release.
- Remove PodSecurityPolicy.

## [0.27.0] - 2025-04-24

### Removed

- Clean up alloy-rules app and configmap because rules are loaded by alloy-logs and alloy-metrics.

## [0.26.1] - 2025-04-23

### Fixed

- Fix golangci-lint v2 problems.

## [0.26.0] - 2025-04-23

### Added

- Add validation webhook to validate the alertmanager config before it is send to the alertmanager.

### Fixed

- Ensure support for loading Prometheus Rules in the Mimir Ruler from workload clusters is only enabled for observability-bundle version 1.9.0 and above (extra query matchers have been added in alloy 1.5.0).

## [0.25.0] - 2025-04-22

### Added

- Add support for loading Prometheus Rules in the Mimir Ruler from workload clusters.

### Changed

- Load Prometheus Rules in the Mimir Ruler via Alloy Metrics instead of Alloy Rules on management clusters.

### Removed

- Remove loading of Prometheus Rules for logs into the Loki Ruler via Alloy Rules as it is now managed by Alloy Logs.

## [0.24.0] - 2025-04-15

### Changed

- Update Silence link in notification-template to point to the new GitOps approach.
- Add `helm.sh/resource-policy: keep` annotation on the grafana organization CRD to prevent it's deletion.

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

[Unreleased]: https://github.com/giantswarm/observability-operator/compare/v0.51.2...HEAD
[0.51.2]: https://github.com/giantswarm/observability-operator/compare/v0.51.1...v0.51.2
[0.51.1]: https://github.com/giantswarm/observability-operator/compare/v0.51.0...v0.51.1
[0.51.0]: https://github.com/giantswarm/observability-operator/compare/v0.50.0...v0.51.0
[0.50.0]: https://github.com/giantswarm/observability-operator/compare/v0.49.0...v0.50.0
[0.49.0]: https://github.com/giantswarm/observability-operator/compare/v0.48.1...v0.49.0
[0.48.1]: https://github.com/giantswarm/observability-operator/compare/v0.48.0...v0.48.1
[0.48.0]: https://github.com/giantswarm/observability-operator/compare/v0.47.0...v0.48.0
[0.47.0]: https://github.com/giantswarm/observability-operator/compare/v0.46.2...v0.47.0
[0.46.2]: https://github.com/giantswarm/observability-operator/compare/v0.46.1...v0.46.2
[0.46.1]: https://github.com/giantswarm/observability-operator/compare/v0.46.0...v0.46.1
[0.46.0]: https://github.com/giantswarm/observability-operator/compare/v0.45.1...v0.46.0
[0.45.1]: https://github.com/giantswarm/observability-operator/compare/v0.45.0...v0.45.1
[0.45.0]: https://github.com/giantswarm/observability-operator/compare/v0.44.0...v0.45.0
[0.44.0]: https://github.com/giantswarm/observability-operator/compare/v0.43.1...v0.44.0
[0.43.1]: https://github.com/giantswarm/observability-operator/compare/v0.43.0...v0.43.1
[0.43.0]: https://github.com/giantswarm/observability-operator/compare/v0.42.0...v0.43.0
[0.42.0]: https://github.com/giantswarm/observability-operator/compare/v0.41.0...v0.42.0
[0.41.0]: https://github.com/giantswarm/observability-operator/compare/v0.40.0...v0.41.0
[0.40.0]: https://github.com/giantswarm/observability-operator/compare/v0.39.0...v0.40.0
[0.39.0]: https://github.com/giantswarm/observability-operator/compare/v0.38.0...v0.39.0
[0.38.0]: https://github.com/giantswarm/observability-operator/compare/v0.37.0...v0.38.0
[0.37.0]: https://github.com/giantswarm/observability-operator/compare/v0.36.0...v0.37.0
[0.36.0]: https://github.com/giantswarm/observability-operator/compare/v0.35.0...v0.36.0
[0.35.0]: https://github.com/giantswarm/observability-operator/compare/v0.34.0...v0.35.0
[0.34.0]: https://github.com/giantswarm/observability-operator/compare/v0.33.1...v0.34.0
[0.33.1]: https://github.com/giantswarm/observability-operator/compare/v0.33.0...v0.33.1
[0.33.0]: https://github.com/giantswarm/observability-operator/compare/v0.32.1...v0.33.0
[0.32.1]: https://github.com/giantswarm/observability-operator/compare/v0.32.0...v0.32.1
[0.32.0]: https://github.com/giantswarm/observability-operator/compare/v0.31.0...v0.32.0
[0.31.0]: https://github.com/giantswarm/observability-operator/compare/v0.30.0...v0.31.0
[0.30.0]: https://github.com/giantswarm/observability-operator/compare/v0.29.0...v0.30.0
[0.29.0]: https://github.com/giantswarm/observability-operator/compare/v0.28.0...v0.29.0
[0.28.0]: https://github.com/giantswarm/observability-operator/compare/v0.27.0...v0.28.0
[0.27.0]: https://github.com/giantswarm/observability-operator/compare/v0.26.1...v0.27.0
[0.26.1]: https://github.com/giantswarm/observability-operator/compare/v0.26.0...v0.26.1
[0.26.0]: https://github.com/giantswarm/observability-operator/compare/v0.25.0...v0.26.0
[0.25.0]: https://github.com/giantswarm/observability-operator/compare/v0.24.0...v0.25.0
[0.24.0]: https://github.com/giantswarm/observability-operator/compare/v0.23.2...v0.24.0
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
