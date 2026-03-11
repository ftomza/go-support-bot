# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.0.3] - 2026-03-11

### Added
- `/client_mode` command for administrators to safely test the user funnel without losing manager privileges.
- Category sorting functionality: added an `Order` field and Up/Down arrows in the WebApp to strictly control inline keyboard button positions.
- Custom React modal for adding new categories in the WebApp, ensuring full compatibility with Telegram Desktop.
- Automated database migration system using `pressly/goose`. Migrations are now embedded (`go:embed`) into the binary and automatically applied on bot startup.

### Changed
- Replaced `window.confirm` with Telegram's native `tg.showPopup` for safe category deletion across all platforms.
- Changed the "Prompt text" input in the WebApp from a single-line `<input>` to a multiline `<textarea>` with auto-resize to preserve line breaks.
- Improved user onboarding flow: the bot now persistently re-sends the category menu if a user types text before selecting a topic.
- Cleaned up `Dockerfile` and `docker-compose.yml` by removing raw SQL initialization scripts (now natively handled by Goose).

### Fixed
- Fixed a critical "React Stale Closure" bug in the WebApp where saving would wipe the configuration due to stale state references (`useRef` implemented).
- Added missing `json` tags to Go structs (`YamlConfig`, `YamlTheme`, `YamlMessages`) to properly map incoming JSON data from the WebApp.

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
