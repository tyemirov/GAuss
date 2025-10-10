# Changelog
All notable changes to this project will be documented in this file.

## [Unreleased]
### Added
- Introduced `WithLogoutRedirectURL` so applications can choose the post-logout redirect target while defaulting to the login page.


## [v0.0.11] - 2025-10-09
### Added
- Introduced YouTube-specific OAuth scopes to simplify sample integrations.

### Changed
- Improved autonomous agent flows and clarified coding guidance.
- Adapted OAuth redirect handling to honor forwarded headers when deployed behind reverse proxies.
- Renamed internal variables to better communicate intent.

### Fixed
- Guarded forwarded redirect helpers against nil base URL values.

### Documentation
- Documented reverse proxy usage for forwarded redirect scenarios.
- Added coding instructions for autonomous agents.

## [v0.0.10] - 2025-07-27
### Added
- Added a YouTube listing demo application with dedicated templates.
- Added an MIT license file for the project.

### Changed
- Reorganized example applications into dedicated `examples` packages and updated module dependencies.
- Improved logging and Chrome compatibility in the YouTube demo flow.

### Documentation
- Documented Google Cloud Console setup steps for the YouTube demo.

## [v0.0.9] - 2025-07-20
### Added
- Added handler tests covering OAuth scope selection.

### Changed
- Removed the legacy `content.sh` helper in favor of the shared `temirov/ctx` utility.

### Fixed
- Ensured OAuth handlers honor the scopes provided by the service configuration.

## [v0.0.8] - 2025-07-20
### Added
- Added a `GetClient` helper to build authenticated HTTP clients.
- Added tests documenting the new client acquisition workflow.

### Changed
- Extended documentation for authenticated calls and cleaned scope definitions.
- Updated ignore rules to exclude credentials and CI workflow artifacts.

## [v0.0.7] - 2025-07-18
### Added
- Added configurability for OAuth scopes and token persistence.
- Added extensive unit tests across handlers, middleware, and services.
- Added a GitHub Actions workflow for running Go tests.
- Added comprehensive GoDoc coverage and integration documentation.

### Changed
- Refactored services to improve testability and dependency injection.

## [v0.0.6] - 2025-03-23
### Added
- Added support for supplying custom login templates through the service constructor.
- Added sample templates demonstrating custom login branding.

### Changed
- Updated services, handlers, and documentation to inject optional login templates.

## [v0.0.5] - 2025-03-12
### Changed
- Replaced hard-coded session values with shared constants across handlers and middleware.

## [v0.0.4] - 2025-03-02
### Added
- Added support for configuring the application base URL when constructing the service.

## [v0.0.3] - 2025-01-26
### Fixed
- Prevented repeated account selection prompts during authentication.

## [v0.0.2] - 2025-01-25
### Added
- Embedded the login template within the binary for simpler deployment.

## [v0.0.1] - 2025-01-25
### Added
- Released the initial GAuss authentication service and dashboard implementation.
- Applied Beer CSS styling to the login and dashboard templates.
- Integrated the shared `temirov/utils` package for environment configuration.

### Changed
- Updated Go module dependencies to the latest available patch versions.
