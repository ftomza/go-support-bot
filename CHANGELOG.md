# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.0.9] - 2026-03-17

### Added
- **NPS Analytics Dashboard**: Introduced a new "Statistics" tab in the Admin WebApp to visualize customer satisfaction.
- **Advanced Metrics Calculation**: The dashboard now calculates the Average Score, Total Votes, Score Distribution (1-5 stars), and the classic Net Promoter Score (NPS) formula directly from the `topic_ratings` table.
- **Manager Leaderboard**: Added a real-time leaderboard showing total votes and average scores grouped by active managers. The backend service layer automatically resolves `manager_id` to actual Telegram names.
- **Dynamic Time Filtering**: Added time range presets (Today, 7 Days, Month) and a custom date picker to filter NPS statistics dynamically on both frontend and database layers.
- **Strict Telegram HTML Validator**: Implemented a custom stack-based HTML validator (`ValidateTelegramHTML` and `ValidateYamlConfig`) to pre-validate all system texts, category prompts, and broadcast messages before saving them to the database. This prevents `Bad Request: can't parse entities` errors caused by unclosed tags or invalid attributes.

### Improved
- **Admin UI Layout**: Reorganized the Admin WebApp navigation into a scrollable horizontal tab bar to accommodate new sections without cluttering the screen.
- **Service Layer Architecture**: Cleaned up the data flow for statistics. The Repository now strictly handles raw database queries, while the Service layer is responsible for data enrichment (mapping manager IDs to names).

## [0.0.8] - 2026-03-16

### Added
- **Mass Broadcast System**: Introduced a robust background worker for sending mass messages to clients with HTML formatting support.
- **Persistent Customers Directory**: Migrated from session-based tracking to a dedicated `customers` table. The bot now maintains a complete, persistent directory of all users who have ever interacted with it.
- **Smart Rate Limiting & Safe Queueing**: The background worker strictly respects Telegram API limits and uses PostgreSQL's `FOR UPDATE SKIP LOCKED` for concurrent-safe queue processing.
- **Dead-letter Queue & Retry Mechanism**: Broadcast history now tracks `sent`, `pending`, and `failed` deliveries. Added a UI capability to easily retry sending messages specifically to failed recipients.
- **Automated Block Detection**: The worker automatically detects if a user has blocked the bot or deleted their account, flagging them as `is_blocked = true` in the database to prevent future API errors.
- **Real-time Developer Alerts**: Panic recovery (`defer recover()`) in the background worker now captures full stack traces and instantly sends HTML-formatted alerts directly to the Telegram DMs of developers (configured via `DeveloperIDs`).
- **Admin WebApp Broadcast UI**: Added a new "Broadcasts" tab in the WebApp featuring live customer search, a smart "Select All" toggle (ignoring blocked users), message input, and detailed history monitoring.
- **Broadcast Unit Tests**: Added comprehensive test coverage for broadcast queue processing, error handling, database status updates, and developer notifications.

### Fixed
- **NPS Event Duplication Bug**: Fixed an issue where clients closing a topic themselves received duplicate NPS prompts and menus. The bot now correctly deduplicates Telegram's `CloseForumTopic` system events based on database state.
- **Callback Router Collision**: Fixed a bug where category navigation buttons absorbed all callbacks (including NPS ratings) by adding strict `cat_` prefix filtering to the handler.

## [0.0.7] - 2026-03-15

### Added
- **NPS (Net Promoter Score) System**: Introduced a customer satisfaction rating feature. Upon ticket closure (either by the client or manager), the bot now prompts the user to rate the support quality from 1 to 5 stars using an inline keyboard.
- **Event-Driven Rating Analytics**: Ratings are now stored in a dedicated, append-only `topic_ratings` PostgreSQL table. This architectural choice prevents data overwriting and allows for historical NPS analytics and time-series reporting.
- **Dynamic Manager Attribution**: Added an `active_manager_id` tracker to `customer_topics`. The bot now smartly detects which specific manager last replied to the customer in the Telegram group. When a rating is given, it is accurately attributed to the actual assisting manager rather than the default category owner (using SQL `COALESCE`).
- **Customizable NPS Texts**: Added `RateService` and `RatingThanks` text fields to the database configuration, making the rating prompts fully editable via the WebApp Admin Panel.
- **NPS Unit Tests**: Expanded the test suite in `support_test.go` to cover inline keyboard generation, rating persistence, and the dynamic manager reassignment logic.

## [0.0.6] - 2026-03-14

### Added
- **Customizable System Texts**: All bot messages and button labels (welcome text, out-of-hours replies, manager notifications) have been extracted from the source code. They are now stored in the database and are fully editable via the React WebApp Admin Panel.
- **Database-backed State Machine**: User sessions (waiting for name input) and out-of-hours notification throttling are now persisted in a new `customer_sessions` PostgreSQL table. This ensures resilience against bot restarts and paves the way for horizontal scaling.
- **CI/CD Test Pipeline**: Added a new GitHub Actions workflow (`test.yml`) to automatically run unit tests with race condition detection and code coverage reporting on every push and pull request.
- **Unit Testing**: Implemented comprehensive unit tests for the core service layer and API endpoints using `testify/mock`, covering complex scenarios like remote topic deletion recovery, navigation logic, and access control.

### Changed
- **Soft Delete for Categories**: Categories are now soft-deleted (`is_active = false`) instead of permanently dropped when updating the configuration via the WebApp. This preserves the history and data integrity of older, closed customer tickets.

### Security
- **Admin API Protection**: Hardened the WebApp backend by adding `X-Telegram-Init-Data` signature validation to all `GET` routes (`/api/config/get`, `/api/managers`). This prevents unauthorized access or data leaks of the bot's configuration.

## [0.0.5] - 2026-03-13

### Added
- **Client-Side Topic Closure**: Clients can now explicitly end their support session using a permanent reply keyboard button. This updates the database, notifies the manager, and physically closes the Telegram forum topic.
- **Customizable Close Button**: Added configuration for the "Close Topic" button text directly in the React WebApp Admin Panel.
- **Manual Language Override**: Managers can now use the `/set_lang <lang_code>` command (e.g., `/set_lang es`) inside a topic to force the AI to translate messages to a specific language, overriding the client's default Telegram language.
- **Resilient Topic Re-creation**: Implemented a failsafe mechanism. If an administrator accidentally deletes a forum topic, the bot will gracefully catch the "message thread not found" error, automatically create a new topic, and deliver the pending message without interruption.
- **Automated GitHub Releases**: Added a GitHub Actions workflow (`release.yml`) to automatically build and publish Go binaries (for Linux, Windows, and macOS) whenever a new version tag (`v*`) is pushed.

### Changed
- **Localized Out-of-Hours Notifications**: The automated "out of office/working hours" reply is now dynamically translated into the client's specific language using the Gemini LLM.

### Fixed
- Fixed an API error caused by empty keyboard buttons for legacy database records by enforcing default fallback values for the `CloseTopicButton` text.
- Fixed a Go interface pointer receiver issue when using `telego.ReplyKeyboardRemove` to hide the client's keyboard upon topic closure.

## [0.0.4] - 2026-03-12

### Added
- **Timezone Support**: Added timezone selection for category working hours. The backend now accurately calculates availability based on the specific location (e.g., `Asia/Dubai`, `Europe/Moscow`), and `tzdata` was included in the Docker image.
- **Category Images**: Administrators can now attach image URLs to categories via the WebApp. The bot dynamically sends these as photo messages with inline keyboards.
- **Smart Navigation**: Added a "🔙 Назад" (Back) button to return to the previous category level, alongside the "🏠 В начало" (Home) button for deep menu hierarchies.
- New Goose database migrations for `timezone` and `image` columns.

### Changed
- Improved the out-of-hours notification text to explicitly display the timezone context to the user (e.g., `09:00-18:00 (Asia/Dubai)`).
- Refactored Telegram menu navigation logic: the bot now deletes the previous message and sends a new one (instead of editing) to seamlessly support transitions between text-only and media (photo) messages.
- Updated React WebApp UI to include timezone dropdowns, image URL inputs, and live image previews.

### Fixed
- Fixed a routing bug where the `/client_mode` command was intercepted by the global catch-all text handler. Strict commands are now correctly prioritized at the top of the handler chain.

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
