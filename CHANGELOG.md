# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog and this project aims to follow Semantic Versioning.

## [Unreleased]

### Added

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
