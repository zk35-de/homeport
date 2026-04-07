# Contributing

Go knowledge assumed. No CLA, no overhead.

## Dev Setup

```bash
git clone https://github.com/zk35-de/homeport
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

### Test structure

`internal/api/handlers_test.go` – unit tests with stub templates (fast, logic-focused).

`internal/api/smoke_test.go` – render tests using the **real** embedded templates via `assets.FS`. These catch struct/template mismatches (e.g. a template field that exists in the partial but not in the handler's data struct). **If you add a field to any template partial, add it to the corresponding handler data struct and to the smoke test assertions.**

`internal/api/api_routes_test.go` – route-level handler tests. Covers CSRF, category/profile options endpoints, and other handler logic that needs a real `http.ResponseWriter`.

`internal/api/auth_flow_test.go` – full auth stack integration tests using `httptest.Server` + `net/http/cookiejar`. Covers login/logout/session lifecycle and CSRF exemptions. Mirrors what a real browser does; use this when touching auth middleware or session logic.

`tests/e2e/` – Playwright end-to-end tests against a running server (`npx playwright test`). **Run these before closing any bug fix that touches the manage UI.** Start the server first (`./homeport`), then run from `tests/e2e/`.

### HTMX 2.x gotchas

**`hx-target` inheritance**: child elements inherit `hx-target` from their parent. A `<select>` inside a `<form hx-target="#some-div">` will swap its own HTMX responses into `#some-div` unless it has an explicit `hx-target="this"`. Always set `hx-target="this"` on any element that makes its own HTMX requests and lives inside a form with a different `hx-target`.

**`HX-Trigger` event dispatch**: In HTMX 2.x, `HX-Trigger` response headers fire events on `document.body`. Elements using `hx-trigger="eventName from:body"` receive them. If you trigger the event from JS (`htmx.trigger(document.body, 'eventName')`), fire it *after* the relevant DOM swap (e.g. in `htmx:afterSwap`), not before.

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
