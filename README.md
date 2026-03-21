# homeport

Self-hosted startpage for your homelab. Replaces Fenrus/Homer/Dashy.

**Why homeport?**
- No config file editing – everything via management Web-UI
- Per-category layout: tiles | list | icons
- Multi-profile: `/` for Markus, `/andrea` for Andrea (no login needed)
- Written in Go – single binary, no runtime, minimal attack surface
- Live status indicators via SSE (no polling)
- Bearer token auth on all API routes

## Stack

| Component | Choice |
|-----------|--------|
| Language | Go 1.23+ |
| Router | chi |
| DB | SQLite (modernc, pure Go, no CGO) |
| Frontend | html/template + prism-ui CSS |
| Port | 8855 |

## Quick Start

```bash
git clone https://git.zk35.de/secalpha/homeport
cd homeport
go build -o homeport ./cmd/homeport
./homeport
```

Open http://localhost:8855

## Environment Variables

| Var | Default | Description |
|-----|---------|-------------|
| `HOMEPORT_PORT` | `8855` | Listen port |
| `HOMEPORT_DB` | `./data/homeport.db` | SQLite DB path |
| `HOMEPORT_TOKEN` | auto-generated | Bearer token for API auth (logged to stderr on first start) |
| `HOMEPORT_CORS` | `*` | Comma-separated allowed CORS origins |

## Routes

### UI
| Route | Description |
|-------|-------------|
| `GET /` | Markus dashboard |
| `GET /andrea` | Andrea dashboard (filtered view) |
| `GET /manage` | Management UI |
| `GET /s/{code}` | URL shortener redirect (public) |

### API (Bearer token required, except `/api/health`)
| Route | Description |
|-------|-------------|
| `GET /api/health` | `{"status":"ok","version":"dev"}` |
| `GET /api/updates` | SSE stream: service_status + widget_refresh events |
| `GET /api/widgets` | List widgets (`?profile=markus`) |
| `POST /api/widgets` | Create widget |
| `GET /api/widgets/{id}` | Get widget |
| `PATCH /api/widgets/{id}` | Update widget |
| `DELETE /api/widgets/{id}` | Delete widget |
| `PATCH /api/widgets/reorder` | Reorder widgets |
| `GET /api/user/preferences` | Get user preferences (`?profile=markus`) |
| `PATCH /api/user/preferences` | Partial update preferences |
| `POST /api/shorten` | Shorten a URL |
| `GET /api/links` | List all short URLs |
| `DELETE /api/links/{code}` | Delete short URL |

## Features

### Service Dashboard
- Categories with configurable layout (tiles / list / icons) and color
- Per-service visibility: `markus` | `andrea` | `all`
- Live status dots via SSE (30s checks)
- Podman container auto-discovery inbox

### Widgets
- **iCal** – Calendar feed (e.g. Abfallkalender), 6h cache, today/tomorrow markers
- **Weather** – Open-Meteo (no API key), 5-day forecast, config: `{"lat":48.13,"lon":11.58,"city_name":"München"}`

### Search Bar
- Configurable search engine per profile
- Bang syntax: `!g` Google, `!d` DuckDuckGo, `!b` Brave, `!gh` GitHub, `!yt` YouTube, `!w` Wikipedia

### URL Shortener
- `/s/{code}` redirects with click counting
- Optional custom codes, managed via `/manage` or API

### Live Updates (`/api/updates` SSE)
Message envelope:
```json
{"type": "service_status", "payload": {"id": 1, "alive": true}}
{"type": "widget_refresh", "payload": {"widget_id": "uuid"}}
```

## Data Model

```
categories       → name, layout (tiles|list|icons), color, sort_order
services         → category_id, name, url, icon, description, status_check, sort_order
visibility       → service_id, profile
service_status   → service_id, alive, last_check
widgets          → type (ical|weather), name, config (json), profile, sort_order
widget_cache     → widget_id, data (json), fetched_at
user_preferences → profile, theme, accent_color, search_engine, background, language, layout
short_urls       → code, url, clicks, created_at
discovery_inbox  → container_id, suggested (json), seen_at, ignored
```

## Deploy

### Docker Compose
```bash
cp deploy/docker-compose.yml .
HOMEPORT_TOKEN=your-secret docker compose up -d
```

### Podman Quadlet (systemd)
```bash
cp deploy/homeport.container ~/.config/containers/systemd/
systemctl --user daemon-reload
systemctl --user start homeport
```

### Build from source
```bash
go build -o homeport ./cmd/homeport
HOMEPORT_TOKEN=your-secret ./homeport
```

## Dev Setup

```bash
go test ./...     # run tests
go build ./...    # compile all packages
```

No Node.js required. Frontend is Go templates + embedded CSS.
