# Gmail TUI

## Overview

`gmail-tui` is a terminal Gmail client built with Go and Bubble Tea. It provides inbox browsing, search, labels, composition, replies, attachment download, and structured logging in a keyboard-first interface.

## Features

- Concurrent inbox fetch with bounded workers and LRU caching.
- Compose, reply, delete, search, label browsing, and read/unread toggling.
- Safe attachment downloads with filename sanitization and deduplication.
- Configurable behavior through TOML and `GMAIL_TUI_*` environment variables.
- Structured JSON logging to `~/.config/gmail-tui/app.log`.
- Pagination, preserved search queries, and compose/reply quality-of-life improvements.

## Prerequisites

- Go `1.23.3` or newer.
- A Google account with Gmail enabled.
- A Google Cloud project with the Gmail API enabled.
- An OAuth desktop client downloaded as `credentials.json`, or pointed to by `GMAIL_TUI_CREDENTIALS`.

## Quick Start

1. Clone the repository.

   ```bash
   git clone https://github.com/rdx40/gmail-tui
   cd gmail-tui
   ```

2. Create OAuth credentials in Google Cloud Console.

   - Enable the Gmail API.
   - Configure the OAuth consent screen.
   - Create an OAuth 2.0 Desktop App client.
   - Download the JSON credentials file.

3. Place the credentials file.

   ```bash
   cp /path/to/credentials.json ./credentials.json
   ```

   Or point the app to a different location:

   ```bash
   export GMAIL_TUI_CREDENTIALS=/path/to/credentials.json
   ```

4. Build the application and run it.

   ```bash
   go build . && ./gmail-tui.exe or ./gmail-tui
   ```

5. On first launch, open the authorization URL shown in the terminal, grant access, and paste the returned code back into the app.

## Configuration

Configuration is loaded in this order:

1. Built-in defaults
2. `~/.config/gmail-tui/config.toml`
3. `GMAIL_TUI_*` environment variables

Example `config.toml`:

```toml
max_results = 10
search_max_results = 30
downloads_dir = "downloads"
max_concurrent = 5
cache_max_size = 500
log_level = "INFO"
```

Supported environment variables:

- `GMAIL_TUI_MAX_RESULTS`
- `GMAIL_TUI_SEARCH_MAX_RESULTS`
- `GMAIL_TUI_DOWNLOADS_DIR`
- `GMAIL_TUI_MAX_CONCURRENT`
- `GMAIL_TUI_CACHE_MAX_SIZE`
- `GMAIL_TUI_LOG_LEVEL`
- `GMAIL_TUI_CREDENTIALS`

## Key Bindings

### Global and Inbox

| Key | Action |
| --- | --- |
| `j` / `k` | Move selection |
| `enter` | Open selected email |
| `c` | Compose |
| `d` | Move email to trash |
| `m` | Toggle read/unread |
| `l` | Open labels |
| `/` | Search |
| `n` | Next page |
| `p` | Previous page |
| `?` | Toggle help |
| `q` | Quit |

### Viewing

| Key | Action |
| --- | --- |
| `b` / `esc` | Back to inbox |
| `r` | Reply |
| `ctrl+d` | Download attachment |

### Compose and Reply

| Key | Action |
| --- | --- |
| `tab` | Next field |
| `shift+tab` | Previous field |
| `ctrl+s` | Send |
| `ctrl+a` | Add attachment |
| `ctrl+x` | Remove last attachment |
| `esc` | Cancel or return |

## Architecture Overview

The application uses a Bubble Tea Model-View-Update loop:

- `main.go` bootstraps config, logging, the Gmail client, and the TUI.
- `api/` owns Gmail API access, OAuth, parsing, validation, and caching.
- `ui/` owns state transitions, commands, rendering, and keyboard handling.

Additional architecture notes and Mermaid diagrams live in [`docs/architecture.md`](docs/architecture.md).

## Testing Instructions

### Automated Checks

If `make` works in your environment, run the full local verification set with:

```bash
make test
make build
make coverage
make integration-test
make lint
```

If you are on Windows or do not have a working `make` setup, use direct Go commands instead.

PowerShell example:

```powershell
New-Item -ItemType Directory -Force -Path .tmp\go-build, .tmp\go-tmp | Out-Null
$env:GOCACHE = "$PWD\.tmp\go-build"
$env:GOTMPDIR = "$PWD\.tmp\go-tmp"

go test ./...
go test -race -count=1 ./...
go test -tags integration ./api/...
go build ./...
go vet ./...
```

Coverage:

```powershell
go test -coverprofile=coverage.out ./...
go test -coverprofile=coverage_api.out ./api
go tool cover -func=coverage_api.out
go tool cover -html=coverage.out -o coverage.html
```

The project expects API package statement coverage to stay at or above `80%`.

### Linting

If `golangci-lint` is installed:

```bash
make lint
```

Direct command:

```bash
golangci-lint run
```

Pre-commit hooks:

```bash
pip install pre-commit
pre-commit install
pre-commit run --all-files
```

The pre-commit configuration expects `gofmt`, `goimports`, `go vet`, and `golangci-lint` to be available locally.

### Manual Smoke Test

Use a test Gmail account if possible.

1. Start the app.

   ```bash
   go run .
   ```

2. Complete the OAuth flow on first launch.

3. Verify inbox behavior.

   - Inbox loads without crashing.
   - `j` / `k` moves selection.
   - `enter` opens a message.
   - `n` and `p` move through pages when more mail exists.
   - `/` searches and empty results show a clear status message.
   - `l` opens labels and selecting a label loads messages.

4. Verify message actions.

   - `m` toggles read/unread.
   - `d` asks for confirmation and moves the message to trash.
   - `ctrl+d` downloads an attachment and shows the resolved save path.

5. Verify compose and reply behavior.

   - `c` opens compose.
   - Sending with an empty `To` field shows `Error: recipient address is required`.
   - Sending with an empty subject shows `Send with no subject? [y/n]`.
   - Invalid addresses are rejected with a status-bar error.
   - Missing or oversized attachments are rejected before send.
   - `r` from an opened email creates a reply with a normalized `Re: ` subject.

6. Verify logs and artifacts.

   - Structured logs are written to `~/.config/gmail-tui/app.log`.
   - Downloaded attachments stay inside the configured downloads directory.

### Recommended Local Test Order

For day-to-day development, this is the fastest useful sequence:

1. `go test ./...`
2. `go test -tags integration ./api/...`
3. `go build ./...`
4. `go vet ./...`
5. `golangci-lint run`
6. `go run .` for a manual smoke test

## OAuth Scopes

The app requests two Gmail scopes:

- `https://www.googleapis.com/auth/gmail.readonly`
  Needed for inbox listing, message viewing, labels, and attachment download.
- `https://www.googleapis.com/auth/gmail.send`
  Needed for compose and reply operations.

If send permission is unavailable, compose and reply stay disabled and the UI shows an informational status message.

## Troubleshooting

- `credentials file not found`
  Confirm that `credentials.json` exists or set `GMAIL_TUI_CREDENTIALS`.
- `failed to decode token (corrupted file removed)`
  Delete the token file if it still exists and re-run the app to start OAuth again.
- `download path escapes downloads directory`
  The requested attachment filename was rejected by the path-safety guard.
- `attachment too large (max 25 MB)`
  Gmail attachments above the 25 MB client-side limit are rejected before send.

More operational guidance lives in [`docs/runbook.md`](docs/runbook.md).

## Contributing

Contribution workflow, branch naming, testing expectations, and review process are documented in [`CONTRIBUTING.md`](CONTRIBUTING.md).
