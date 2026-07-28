# Gmail TUI

`gmail-tui` is a keyboard-driven terminal client for Gmail, written in Go on top of Bubble Tea's Model-View-Update framework. It talks to the Gmail API directly over OAuth2 (no IMAP/SMTP), and handles inbox browsing, search, labels, compose/reply, attachment transfer, and read/unread state entirely from the terminal. The project is structured as a small two-package system — a Gmail API client (`api/`) and a terminal UI (`ui/`) — with unit, property-based, and integration tests covering both.

## Features

- Concurrent inbox fetch using bounded worker goroutines, backed by an LRU cache to avoid redundant API calls.
- Compose, reply (with `Re:` subject normalization and thread headers), delete, search, label browsing, read/unread toggling.
- Attachment download with filename sanitization, path-escape protection, and dedup.
- Layered configuration: built-in defaults → TOML file → `GMAIL_TUI_*` env vars.
- Structured JSON logging to `~/.config/gmail-tui/app.log`.
- Pagination and preserved search state across navigation.

## Architecture

Bubble Tea enforces a strict Model-View-Update loop: user input and async API results become messages, `Update` produces a new model plus any follow-up commands, `View` renders it. The API package is kept UI-agnostic — it returns plain data/errors, and `ui/commands.go` wraps calls as `tea.Cmd`s so the network never blocks the render loop.

```
┌──────────────────────────────┐
│            main.go            │
│  load config → init logger    │
│  → init Gmail client → run TUI│
└───────────────┬────────────────┘
                │
        ┌───────┴────────┐
        ▼                 ▼
┌───────────────┐   ┌─────────────────────────────┐
│     api/       │   │            ui/               │
│ OAuth, Gmail   │◄──┤ Model-View-Update (Bubble Tea)│
│ REST calls,    │   │  model.go   - app state        │
│ parsing,       │   │  update.go  - msg handling      │
│ validation,    │   │  commands.go- async tea.Cmd     │
│ LRU cache,     │   │  view.go    - rendering          │
│ JSON logger    │   │  keys.go    - keybindings        │
└───────┬────────┘   └─────────────────────────────┘
        │
        ▼
  Gmail REST API (google.golang.org/api/gmail/v1)
```

Async operations (fetch, send, download) run as `tea.Cmd`s returning a result message; `Update` never calls the network synchronously.

## Tech Stack

- **Language:** Go 1.23.3
- **TUI framework:** `charmbracelet/bubbletea`, `bubbles`, `lipgloss`
- **Gmail access:** `google.golang.org/api` (Gmail v1), `golang.org/x/oauth2`
- **Config:** `BurntSushi/toml`
- **Testing:** stdlib `testing`, `pgregory.net/rapid` (property-based tests, vendored under `third_party/rapid`)
- **Tooling:** `golangci-lint`, `pre-commit` (gofmt, goimports, go vet), GitHub Actions CI

## Installation

Prerequisites: Go 1.23.3+, a Google account with Gmail enabled, and a Google Cloud project with the Gmail API enabled.

```bash
git clone https://github.com/rdx40/gmail-tui
cd gmail-tui
go build .
```

Create an OAuth 2.0 Desktop App client in Google Cloud Console, download it as `credentials.json`, and place it in the project root (or point `GMAIL_TUI_CREDENTIALS` at it).

## Configuration

Resolved in order: built-in defaults → `~/.config/gmail-tui/config.toml` → `GMAIL_TUI_*` env vars.

```toml
max_results = 10
search_max_results = 30
downloads_dir = "downloads"
max_concurrent = 5
cache_max_size = 500
log_level = "INFO"
inbox_query = "in:inbox category:primary"
```

Equivalent env vars: `GMAIL_TUI_MAX_RESULTS`, `GMAIL_TUI_SEARCH_MAX_RESULTS`, `GMAIL_TUI_DOWNLOADS_DIR`, `GMAIL_TUI_MAX_CONCURRENT`, `GMAIL_TUI_CACHE_MAX_SIZE`, `GMAIL_TUI_LOG_LEVEL`, `GMAIL_TUI_INBOX_QUERY`, `GMAIL_TUI_CREDENTIALS`.

`inbox_query` controls what the main list shows — e.g. `in:inbox` to include all categories, not just Primary.

## Running locally

```bash
go run .
```

On first launch the app opens your browser for OAuth authorization and captures the redirect on a local loopback port — no copy-paste needed. If a local listener can't be started (headless environments), it falls back to printing the URL and reading the code from the terminal. The token is cached locally after that.

For day-to-day verification:

```bash
go test ./...
go test -tags integration ./api/...
go build ./...
go vet ./...
golangci-lint run
```

(`make test`, `make build`, `make lint`, `make coverage` wrap the same commands if `make` is available.)

## Example usage

| Key | Action |
| --- | --- |
| `j` / `k` | Move selection |
| `enter` | Open selected email |
| `/` | Search |
| `c` | Compose |
| `r` | Reply |
| `d` | Move to trash |
| `a` | Archive (remove from inbox) |
| `m` | Toggle read/unread |
| `l` | Labels |
| `R` | Refresh inbox |
| `ctrl+d` | Download attachment |
| `n` / `p` | Next / previous page |
| `?` | Help |
| `q` | Quit |

Sending with an empty recipient is rejected inline (`Error: recipient address is required`); an empty subject prompts `Send with no subject? [y/n]` rather than failing silently.

## Project Structure

```
.
├── main.go            # composition root: config → logger → client → tea.Program
├── api/                # Gmail API layer, no UI dependency
│   ├── auth.go          # OAuth2 flow, token persistence
│   ├── client.go         # Gmail REST calls (list, get, send, trash, labels, attachments)
│   ├── cache.go          # bounded LRU cache for fetched messages
│   ├── config.go         # layered config load (defaults/TOML/env)
│   ├── parse.go           # MIME/body parsing, HTML stripping
│   ├── validate.go        # address/attachment/size validation
│   ├── errors.go           # typed error values used across the package
│   ├── logger.go            # structured JSON logger
│   ├── types.go              # shared data types (Message, Attachment, ...)
│   ├── interfaces.go          # client interface for UI-side mocking
│   └── *_test.go, property_test.go, integration_test.go
├── ui/                 # Bubble Tea TUI layer
│   ├── model.go          # application state
│   ├── update.go          # message handling / state transitions
│   ├── commands.go         # async tea.Cmd wrappers around api/ calls
│   ├── view.go              # rendering
│   ├── keys.go               # keybinding definitions
│   └── *_test.go, property_test.go
├── third_party/rapid/  # vendored pgregory.net/rapid (property-based testing)
├── images/             # screenshots used in this README
├── .github/workflows/  # CI: build, test, lint
└── .golangci.yml, .pre-commit-config.yaml
```

## Design Decisions

- **Bubble Tea / MVU over ad-hoc terminal rendering:** a strict message-driven update loop keeps state transitions testable (`update_test.go`) without a running terminal, and makes async Gmail calls composable as `tea.Cmd` instead of littering goroutines through the render path.
- **`api/` has zero UI imports:** the Gmail client is usable standalone and unit-testable without spinning up Bubble Tea. `interfaces.go` exists specifically so `ui/` can mock the client in tests.
- **LRU cache in front of Gmail REST calls:** Gmail API quota and latency make repeated identical fetches (e.g., re-rendering the same inbox page) expensive; a bounded in-memory cache trades a small memory cost for fewer round trips.
- **Layered config (defaults → TOML → env):** covers both the "just run it" case and containerized/CI use where env vars are the natural override mechanism, without forcing either.
- **Requested scopes, checked against what's actually granted:** the app requests `gmail.readonly`, `gmail.send`, and `gmail.modify` (modify is required for trash/read-toggle). Google's consent screen lets a user deselect individual scopes, so the granted set can be narrower than requested — `HasSendScope()` reads the scope Google actually returned on the token and disables compose/reply rather than failing the whole app if send wasn't granted.

## Challenges

- Gmail messages arrive as nested multipart MIME with inconsistent charset/encoding; `parse.go` and its large test suite (`parse_test.go`, `property_test.go`) exist because plain-text extraction and HTML stripping had many edge cases (empty parts, base64 padding, nested alternatives).
- Attachment handling needed explicit path-escape and size-limit checks (`download path escapes downloads directory`, 25 MB cap) since filenames come from an untrusted remote source.
- Keeping the Gmail client fully decoupled from Bubble Tea required deliberate interface boundaries (`interfaces.go`) so async commands could be tested without a live terminal or real network.

## Future Improvements

- Multi-account support and account switching.
- Offline/local message index for faster search.
- Configurable keybindings instead of a fixed scheme.
- Thread-level (conversation) view rather than flat message list.

## Screenshots

| Inbox | Compose |
| --- | --- |
| ![Inbox](images/inbox.png) | ![Compose](images/compose.png) |

| Attach (compose) | Attach (received) |
| --- | --- |
| ![Attach send](images/attach_send.png) | ![Attach receive](images/attach_rec.png) |

