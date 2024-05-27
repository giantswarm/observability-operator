# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Do nothing if mimir is disabled to avoid deleting prometheus-meta-operator managed resources.

## [0.0.3] - 2024-05-24

### Changed

- Manage prometheus-agent configs

## [0.0.2] - 2024-04-08

### Fixed

- Fix `CiliumNetworkPolicy` to allow cluster and world access (opsgenie)

## [0.0.1] - 2024-04-08

### Added

- Initialize project and create heartbeat for the installation.

[Unreleased]: https://github.com/giantswarm/observability-operator/compare/v0.0.3...HEAD
[0.0.3]: https://github.com/giantswarm/observability-operator/compare/v0.0.2...v0.0.3
[0.0.2]: https://github.com/giantswarm/observability-operator/compare/v0.0.1...v0.0.2
[0.0.1]: https://github.com/giantswarm/observability-operator/releases/tag/v0.0.1
