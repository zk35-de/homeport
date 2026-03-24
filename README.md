# homeport

Self-hosted startpage for your homelab. Replaces Fenrus/Homer/Dashy.

## Screenshots

| Dashboard (dark) | Dashboard (light) |
|---|---|
| ![Dashboard dark](docs/screenshots/dashboard-dark.png) | ![Dashboard light](docs/screenshots/dashboard-light.png) |

| Search spotlight | Manage UI |
|---|---|
| ![Search](docs/screenshots/search-spotlight.png) | ![Manage](docs/screenshots/manage.png) |

<p align="center">
  <img src="docs/screenshots/mobile.png" alt="Mobile view" width="300">
</p>

**Why homeport?**
- No config file editing – everything via management Web-UI
- Multi-profile: `/` default, `/{slug}` per user (no login needed)
- **Multi-page layouts**: Work / Personal / Hobby tabs with keyboard shortcuts
- Per-category layout: tiles | list | icons + collapsible + grid span
- Written in Go – single binary, no runtime, minimal attack surface
- Live status indicators via SSE
- Click-tracking per profile → optional smart sort by usage

## Stack

| Component | Choice |
|-----------|--------|
| Language  | Go 1.23+ |
| Router    | chi v5 |
| DB        | SQLite (modernc, pure Go, no CGO) |
| Frontend  | html/template + HTMX + prism-ui CSS |
| Port      | 8855 |

## Quick Start

```bash
git clone https://git.zk35.de/secalpha/homeport
cd homeport
go build -o homeport ./cmd/homeport
./homeport
```

Open http://localhost:8855, configure at http://localhost:8855/manage

## Environment Variables

| Var | Default | Description |
|-----|---------|-------------|
| `HOMEPORT_PORT` | `8855` | Listen port |
| `HOMEPORT_DB` | `./data/homeport.db` | SQLite DB path |
| `HOMEPORT_TOKEN` | auto-generated | Bearer token for API auth (logged on first start) |
| `HOMEPORT_CORS` | `*` | Comma-separated allowed CORS origins |
| `HOMEPORT_BACKUP_DIR` | `./backups` | Directory for scheduled backups |
| `HOMEPORT_BACKUP_INTERVAL` | `` | Go duration, e.g. `24h` (empty = disabled) |
| `HOMEPORT_BACKUP_MAX_KEEP` | `7` | Number of backup files to keep |
| `HOMEPORT_AUTH` | `false` | Enable session-based login (`true`/`false`) |
| `HOMEPORT_PUBLIC_PROFILE` | `` | Profile slug accessible without login (e.g. `public`) |
| `HOMEPORT_SESSION_DAYS` | `30` | Cookie lifetime in days |

## Routes

### UI
| Route | Description |
|-------|-------------|
| `GET /` | Default profile dashboard |
| `GET /{slug}` | Profile dashboard by slug |
| `GET /manage` | Management UI |
| `GET /r/{id}?p={profile}` | Click-tracking redirect to service URL |

### Auth
| Route | Description |
|-------|-------------|
| `GET /login` | Login page |
| `POST /login` | Submit credentials (form: `profile`, `password`) |
| `POST /logout` | Invalidate session cookie |
| `GET /manage/auth` | Password management (admin only) |
| `POST /manage/auth/password` | Set password for a profile (form: `profile`, `password`) |
| `POST /manage/auth/password/delete` | Remove password for a profile |

### Management (HTMX, no auth)
| Route | Description |
|-------|-------------|
| `POST /manage/service` | Add service |
| `POST /manage/category` | Add category |
| `POST /manage/widget` | Add widget |
| `POST /manage/category/{id}/sortmode/{mode}` | Toggle sort mode (manual\|usage) |
| `POST /manage/category/{id}/span/{span}` | Set grid column span (1\|2\|3) |
| `POST /manage/profile` | Add profile |
| `DELETE /manage/profile/{slug}` | Delete profile |
| `POST /manage/profile/{slug}/default` | Set default profile |
| `GET /manage/page-list?profile=` | Page list partial (HTMX) |
| `POST /manage/page` | Add page (form: profile, name, icon) |
| `DELETE /manage/page/{id}` | Delete page |
| `PATCH /manage/page/{id}` | Update page name/icon |
| `POST /manage/sort/page/{id}/{direction}` | Reorder page (up\|down) |
| `POST /manage/category/{id}/page/{pageID}` | Assign category to page (0=unassigned) |
| `GET /manage/backup` | Download SQLite snapshot |
| `POST /manage/restore` | Upload & restore backup |
| `GET /manage/analytics` | Click analytics (top-25 per profile) |

### API (Bearer token required, except `/api/health` and `/api/search`)
| Route | Description |
|-------|-------------|
| `GET /api/health` | `{"status":"ok"}` |
| `GET /api/search?q=&profile=` | Full-text search across services |
| `GET /api/updates` | SSE: service_status events |
| `GET /api/widgets` | List widgets |
| `POST /api/widgets` | Create widget |
| `PATCH /api/widgets/{id}` | Update widget |
| `DELETE /api/widgets/{id}` | Delete widget |
| `PATCH /api/widgets/reorder` | Reorder widgets |
| `GET /api/user/preferences` | Get preferences |
| `PATCH /api/user/preferences` | Partial update preferences |
| `POST /api/todos` | Add todo item |
| `POST /api/todos/{id}/toggle` | Toggle todo done/undone |
| `DELETE /api/todos/{id}` | Delete todo |
| `POST /api/widgets/{id}/bookmark` | Add bookmark link (form: name, url) |
| `DELETE /api/widgets/{id}/bookmark/{idx}` | Remove bookmark link by index |
| `PUT /api/notes/{id}` | Save note content `{"content":"..."}` |
| `GET /api/favicon?url=` | Proxy favicon fetch |

## Features

### Service Dashboard
- Categories with layout (tiles / list / icons), color, collapsible, grid span (full / half / third)
- Per-service visibility per profile
- Live status dots via SSE (30s checks)
- Click-tracking per profile; 📊 toggle per category = sort by usage
- Podman container auto-discovery inbox
- **Auto-Discovery Sources:** NPM and Docker TCP backends, configurable per source

### Login System
- Opt-in via `HOMEPORT_AUTH=true` (default off – backward compatible)
- bcrypt password hashing; first profile to get a password becomes admin
- Session cookie: HttpOnly, SameSite=Lax, configurable lifetime
- CLI password reset: `homeport passwd <profile>` (reads from stdin)
- Admin-only: `/manage/auth` to manage all profile passwords
- `HOMEPORT_PUBLIC_PROFILE=<slug>` – one profile accessible without login

### Auto-Discovery
- Configured sources in `/manage` → "Auto-Discovery Quellen"
- **Nginx Proxy Manager:** REST API, `identity:secret` auth, auto token-refresh
- **Docker TCP:** `GET /containers/json`, labels `homeport.name` / `homeport.url` / `homeport.icon` / `homeport.description`
- Per-source scan interval (seconds), enable/disable toggle
- Found services appear in Discovery Inbox for manual review (accept/ignore)

### Widgets (all cached server-side)
| Type | Config | Cache |
|------|--------|-------|
| **iCal** | `{"url":"..."}` | 6h |
| **Weather** | `{"lat":48.13,"lon":11.58,"city_name":"München"}` | 6h |
| **RSS** | `{"url":"...","max":10}` | 30min |
| **Clock** | `{"mode":"digital\|analog\|countdown","timezone":"Europe/Berlin"}` | live |
| **Todo** | none | DB (live) |
| **Bookmarks** | `{"layout":"grid","links":[{"name":"...","url":"..."}]}` | DB (live) |
| **Notes** | none | DB (live) |

### Search Bar
- **Spotlight search:** results appear inline while typing (DOM-based, no server round-trip)
- **Search engine dropdown** directly on the bar: DuckDuckGo, Google, Brave, Startpage, Bing
- Selected engine persisted per profile (`POST /manage/settings/search`)
- `Enter` without a selected result → external search in new tab
- Bang syntax: `!g` Google · `!d` DuckDuckGo · `!b` Brave · `!gh` GitHub · `!yt` YouTube · `!w` Wikipedia
- `Ctrl+K` / `/` focuses the search bar from anywhere; `Escape` clears and closes

### Multi-Page Layouts
- Create named pages per profile (Work / Personal / Hobby / …)
- Tab bar appears automatically when pages exist
- Assign categories to pages via manage UI → "Page" dropdown
- Unassigned content (Page 0) appears on **all** tabs
- **Keyboard shortcuts:** `0` / `` ` `` = All, `1`–`9` = page 1–9
- Active tab persisted in `localStorage` per profile
- No page reload – client-side show/hide

### Appearance
- Themes: dark / light / system
- Accent color picker
- Custom CSS override
- Background modes: Aurora (animated) / Tageszeit (morning/day/evening/night gradient) / None

### Analytics
- `GET /manage/analytics` – Top-25 meist geklickte Dienste pro Profil
- Profil-Filter per Dropdown
- Klicks werden automatisch via `/r/{id}?p={profile}` erfasst

### Backup & Restore
- Manual download: `GET /manage/backup` (SQLite VACUUM INTO snapshot)
- Scheduled: set `HOMEPORT_BACKUP_INTERVAL=24h`
- Restore via file upload (validated before swap)

## Data Model

```
profiles         → slug, name, is_default, sort_order
pages            → profile, name, icon, sort_order
categories       → name, layout, color, sort_order, col_span, sort_mode, page_id→pages
services         → category_id, name, url, icon, description, status_check, sort_order
visibility       → service_id, profile
service_status   → service_id, alive, last_check
service_clicks   → service_id, profile, click_count, last_clicked
widgets          → type, name, config (json), profile, sort_order, page_id→pages
widget_cache     → widget_id, data (json), fetched_at
todos            → widget_id, text, done, due_date, sort_order
notes            → widget_id, content, updated_at
user_preferences → profile, theme, accent_color, search_engine, background_mode, custom_css
user_settings    → profile, search_engine
discovery_inbox   → container_id, external_id, suggested (json), seen_at, ignored, source_id→discovery_sources
discovery_sources → type, name, url, token, enabled, interval, created_at
user_auth         → profile, password (bcrypt), is_admin, created_at, updated_at
sessions          → token, profile, expires_at, created_at
```

## CI/CD

Gitea Actions workflows in `.gitea/workflows/`:
- `ci.yml` – build + test + vet + govulncheck on every push to `main`
- `release.yml` – linux/amd64 + linux/arm64 binaries + checksums on `v*` tags

## Deploy

### Build from source
```bash
go build -o homeport ./cmd/homeport
HOMEPORT_TOKEN=your-secret ./homeport
```

### Podman Quadlet (systemd)
```bash
cp deploy/homeport.container ~/.config/containers/systemd/
systemctl --user daemon-reload
systemctl --user start homeport
```

### Docker Compose
```bash
cp deploy/docker-compose.yml .
HOMEPORT_TOKEN=your-secret docker compose up -d
```

## Security

- **Open Redirect** protection on `/r/{id}`: non-http(s) target URLs (e.g. `javascript:`, `data:`) → 404
- **SSRF** protection on `/api/favicon`: private/loopback IPs (RFC1918, 127.x, 169.254.x) blocked via DNS resolution → 403
- **XSS**: Go `html/template` auto-escaping enforced; `javascript:` href sanitized to `#ZgotmplZ`
- API routes under `/api/*` require Bearer token auth (except `/api/health`, `/api/search`, `/api/favicon`)
- **Session auth** (`HOMEPORT_AUTH=true`): bcrypt + secure cookie; no session → redirect to `/login`
- **CSRF**: Double-Submit Cookie (`hp_csrf`); all POST/PATCH/DELETE without valid token → 403; HTMX injects token automatically
- **Rate-limiting** on `/login`: 5 attempts / 5 min per IP, then 2s delay; X-Forwarded-For aware
- **Security Headers** on every response: `Content-Security-Policy` (`default-src 'self'`), `X-Content-Type-Options: nosniff`, `X-Frame-Options: SAMEORIGIN`, `Referrer-Policy: strict-origin-when-cross-origin`
- **SSRF** protection on `/api/favicon`: scheme validation (http/https only); private IPs allowed for homelab targets
- **Supply chain**: `go.sum` pinned, `govulncheck` in CI, embedded JS assets (htmx, sse.js) served locally

## Dev

```bash
go test ./...              # run tests
go test ./... -cover       # with coverage (api ≥30%, db ≥55%)
go build ./...             # compile all packages
```

**E2E tests** (requires running server on port 8855):

```bash
cd tests/e2e
npm install
npx playwright test
```

No Node.js required for the app itself. Frontend is Go templates + HTMX + embedded CSS.
