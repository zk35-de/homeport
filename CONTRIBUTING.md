# Contributing

Go-Kenntnisse werden vorausgesetzt. Kein CLA, kein Overhead.

## Dev Setup

```bash
git clone https://git.zk35.de/secalpha/homeport
cd homeport
go mod download
go build -o homeport ./cmd/homeport
./homeport
```

Open http://localhost:8855 – SQLite DB wird automatisch in `./data/` angelegt.

## CI lokal ausführen

```bash
go build ./...
go test ./...
go vet ./...

# Vulnerability check (einmalig installieren):
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

CI läuft auf jedem Push zu `main` und auf PRs – dieselben Schritte.

## Branches & Commits

Branch vom `main` abzweigen:

```
fix/kurze-beschreibung
feat/kurze-beschreibung
docs/kurze-beschreibung
```

Commit-Format: `<typ>: <was>` – Beispiele:

```
fix: correct status check timeout
feat: add rss widget
docs: extend deploy section
```

Kein Ticket-Prefix nötig, aber ein Issue-Verweis in der PR-Beschreibung hilft.

## Pull Requests

- PR gegen `main`
- Beschreibung: was und warum, kein Roman
- Tests für neue Funktionalität; bestehende Tests dürfen nicht brechen
- `go vet` + `govulncheck` müssen sauber sein

## Issue Labels

| Label | Bedeutung |
|-------|-----------|
| `bug` | Etwas funktioniert nicht wie erwartet |
| `feat` | Neues Feature oder Erweiterung |
| `docs` | Dokumentation |
| `refactor` | Code-Änderung ohne Verhaltensänderung |
| `security` | Sicherheitsrelevant |
