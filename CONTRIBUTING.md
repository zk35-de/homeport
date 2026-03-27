# Contributing

Go knowledge assumed. No CLA, no overhead.

## Dev Setup

```bash
git clone https://github.com/secalpha/homeport
cd homeport
go mod download
go build -o homeport ./cmd/homeport
./homeport
```

Open http://localhost:8855 – SQLite DB is created automatically in `./data/`.

## Run CI Locally

```bash
go build ./...
go test ./...
go vet ./...

# Vulnerability check (install once):
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

CI runs on every push to `main` and on PRs – same steps.

## Branches & Commits

Branch off `main`:

```
fix/short-description
feat/short-description
docs/short-description
```

Commit format: `<type>: <what>` – examples:

```
fix: correct status check timeout
feat: add rss widget
docs: extend deploy section
```

No ticket prefix required, but an issue reference in the PR description helps.

## Pull Requests

- PR against `main`
- Description: what and why, no novel
- Tests for new functionality; existing tests must not break
- `go vet` + `govulncheck` must be clean

## Issue Labels

| Label | Meaning |
|-------|---------|
| `bug` | Something doesn't work as expected |
| `feat` | New feature or enhancement |
| `docs` | Documentation |
| `refactor` | Code change without behaviour change |
| `security` | Security-relevant |
