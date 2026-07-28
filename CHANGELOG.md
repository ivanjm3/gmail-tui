# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog and this project aims to follow Semantic Versioning.

## [Unreleased]

### Added

- Archive (`a`) removes the selected or open email from the inbox.
- Refresh (`R`) reloads the inbox from the first page.
- List title reflects context: `Inbox`, `Search: <query>`, or the label name.

### Changed

- HTML bodies now keep line structure (`<br>`, `</p>`, headings become newlines) and drop `<script>`/`<style>` content entirely.
- Outgoing subjects are RFC 2047 encoded, all user-supplied headers are CRLF-sanitized, and attachment filenames use RFC 2231-safe `Content-Disposition` formatting.
- Snippets are HTML-entity decoded and truncated on rune boundaries.
- Status messages now always auto-clear; `q`/`ctrl+c` quits from the loading screen.

### Fixed

- Pagination: the initial inbox fetch discarded the next-page token, so `n` never advanced; `p` also refetched the current page instead of the previous one. Both work now, and search/label results reset stale inbox pagination state.
- Reply `References` header now appends the replied message's `Message-ID` per RFC 5322, fixing thread grouping in strict clients.
- Toggling read/unread while viewing an email updates the open email, not just the list row.
- Cache reads return copies and unread updates happen under the cache lock, removing a data race with concurrent fetch workers.

- Configuration loading from TOML and `GMAIL_TUI_*` environment variables.
- Structured JSON logging and a bounded LRU email cache.
- Pagination, improved search UX, compose/reply validation, and reply threading support.
- Unit, property, and integration test coverage for the API and UI packages.
- CI workflow, pre-commit hooks, linting configuration, and project documentation.

### Changed

- Replaced broad Gmail scope usage with readonly and send scopes.
- Hardened attachment downloads, token persistence, and parser edge-case handling.
- Improved compose and reply flows with validation and no-subject confirmation.

### Fixed

- Consistent error-prefix handling in the status bar.
- Safer HTML stripping, MIME recursion handling, and body decode fallbacks.
- Correct reply subject normalization and Gmail threading headers.
