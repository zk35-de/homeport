# homeport

Self-hosted startpage for your homelab. Replaces Fenrus/Homer/Dashy.

**Why homeport?**
- No config file editing – everything via management Web-UI
- Per-category layout: tiles | list | icons
- Multi-profile: `/` for Markus, `/andrea` for Andrea (no login)
- Written in Go – single binary, no runtime, minimal attack surface
- SSE status checks (live up/down indicators without polling)

## Stack

| Component | Choice |
|-----------|--------|
| Language | Go 1.23+ |
| Router | chi |
| DB | SQLite (modernc, pure Go, no CGO) |
| Frontend | html/template + HTMX |
| CSS | prism-ui (coming) / embedded minimal CSS |
| Port | 8854 |

## Run

```bash
go build -o homeport .
./homeport
```

Environment variables:

| Var | Default | Description |
|-----|---------|-------------|
| `HOMEPORT_PORT` | `8854` | Listen port |
| `HOMEPORT_DB` | `./data/homeport.db` | SQLite DB path |

## Routes

| Route | Description |
|-------|-------------|
| `GET /` | Markus dashboard |
| `GET /andrea` | Andrea dashboard (filtered view) |
| `GET /manage` | Management UI |
| `POST /manage/category` | Add category (HTMX partial) |
| `POST /manage/service` | Add service (HTMX partial) |
| `DELETE /manage/category/{id}` | Delete category |
| `DELETE /manage/service/{id}` | Delete service |
| `POST /manage/sort/category/{id}/{dir}` | Reorder category |
| `POST /manage/sort/service/{id}/{dir}` | Reorder service |
| `GET /status/stream` | SSE stream for live status |

## Data model

```
categories  → name, layout (tiles|list|icons), color, sort_order
services    → category_id, name, url, icon, description, status_check, sort_order
visibility  → service_id, profile (markus|andrea)
service_status → service_id, alive, last_check
```

## Planned

- [ ] prism-ui integration (#7)
- [ ] Podman label auto-discovery as inbox (#13)
- [ ] Clone-for-Andrea button (#10)
- [ ] iCal widget – Abfallkalender (#11)
- [ ] PWA support (#12)
- [ ] Podman Quadlet deployment (#8)
