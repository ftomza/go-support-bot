# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https.keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https.semver.org/spec/v2.0.0.html).

## [0.0.2] - 2026-03-09

### Added
- `CHANGELOG.md` file.
- Panic recovery middleware to the admin API endpoints for better stability.

### Changed
- Replaced magic number for throttle duration with a constant in `internal/app/service/support.go`.
- Added logging for ignored errors in `internal/app/service/support.go`.
- Handled the error from `srv.ListenAndServe` in `cmd/go-support-bot/main.go`.
- Made the config path configurable via a command-line flag in `internal/app/config/config.go` and `cmd/go-support-bot/main.go`.
- Refactored admin-related API handlers into `internal/app/endpoints/admin.go`.

## [0.0.1] - 2026-03-05

### Added

- Initial version of the `go-support-bot`.
- Features include multi-language support, topic-based discussions, flexible configuration, and out-of-office replies.
- Basic project structure with separation of concerns.
- `README.md` with detailed instructions.
