# Contributing

## Fork and Clone

1. Fork the repository on GitHub.
2. Clone your fork locally.

   ```bash
   git clone https://github.com/<your-account>/gmail-tui
   cd gmail-tui
   ```

3. Add the upstream remote.

   ```bash
   git remote add upstream https://github.com/rdx40/gmail-tui
   ```

## Branch Naming

Use short, descriptive branch names:

- `feature/<short-description>`
- `fix/<short-description>`
- `docs/<short-description>`
- `chore/<short-description>`

Examples:

- `feature/reply-threading`
- `fix/attachment-path-guard`
- `docs/testing-guide`

## Commit Message Format

Prefer concise imperative commit subjects.

Recommended format:

```text
type(scope): summary
```

Examples:

- `fix(ui): validate reply recipients before send`
- `test(api): add oauth token persistence coverage`
- `docs(readme): document oauth scopes`

## Running Tests

Before opening a pull request, run:

```bash
make test
make build
make coverage
go vet ./...
```

If you changed the API layer, also run:

```bash
make integration-test
```

If you use pre-commit:

```bash
pre-commit run --all-files
```

## Pull Request Review Process

1. Keep pull requests focused on one change area.
2. Include test coverage for new logic or regressions.
3. Update documentation when behavior, commands, or setup steps change.
4. Link the relevant spec or issue when applicable.
5. Address review comments with follow-up commits unless the reviewer asks for a squash.

## Keeping Your Branch Current

```bash
git fetch upstream
git rebase upstream/main
```
