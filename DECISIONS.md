# homeport – Decisions

Source of truth for vision and design decisions.

---

## Vision

> **The most comfortable self-hosted startpage – per-user views without editing config files, with spotlight search.**

### Why homeport exists

Existing self-hosted startpages have at least one of these problems:
1. **Config files** – editing bookmarks in YAML/JSON is not UX, that's system administration
2. **No real multi-user** – everyone sees the same thing, or separation is cosmetic only
3. **Supply chain** – unnecessary dependencies, bloated images, no digest pins
4. **Feature creep** – monitoring widgets, weather APIs, system metrics – everything except what a startpage should be

homeport solves these four problems. Nothing more, nothing less.

---

## Users

**Primary user (admin)** – manages services, uses many different categories frequently
**Household members / colleagues** – different interests, should not see each other's links

Both use the same homeport instance but see their own tailored view.

---

## Killer Features (differentiators)

### 1. Multi-user with profile-based visibility
Services are created once (by admin) and can then be selectively shown per profile.
No duplicate maintenance, no sync — one service entry, any number of visibility assignments.
Each profile only sees what's relevant to them.

### 2. Spotlight search for local bookmarks
Keyboard shortcut (`Ctrl+K` / `/`) → search field → type → navigate directly.
DOM-based, no server round-trip. Searches your own bookmarks only, not the web.
Bang syntax for external searches: `!g`, `!d`, `!gh`, `!yt`, `!w` and more.

### 3. Sort by usage within category
Click count per service per profile → frequently used links move to the top.
Automatic, no manual reordering needed. Per profile — one user's usage does not affect another's order.
Click analytics (top-25) available in `/manage/analytics`.

### 4. Zero config-file editing
Everything via the UI. No YAML, no JSON, no restart after changes.
Onboarding = open URL, create profile, go.

### 5. Auto-discovery
homeport detects new services automatically from configured sources:
- **Nginx Proxy Manager** – REST API
- **Traefik** – HTTP provider
- **Docker/Podman TCP** – container labels (`homeport.name`, `homeport.url` etc.)

Discovered services land in an inbox for manual review — nothing is added automatically.
The decision stays with the user; discovery just removes the typing.

---

## Status Check

Bookmarks can have an optional status check (is the service reachable?).
Result: green/red glow on the service icon, updated every 30 seconds via SSE.

**No system monitoring.** homeport does not show CPU load, free disk space, or VM memory usage.
Other tools (Grafana, Uptime Kuma) do that better.
homeport is a startpage, not a dashboard.

---

## Supply Chain (non-negotiable)

- Minimal dependencies — every new dependency needs a justification
- Base image pinned by digest in Containerfile
- `govulncheck` in CI
- No CDN — htmx, sse.js embedded locally, fully self-contained

---

## Deployment Model

homeport is designed for trusted networks (homelab, private LAN).
Do not expose directly to the internet. For remote access: reverse proxy with TLS + `HOMEPORT_AUTH=true`.

---

## Deliberate non-decisions

| What | Why not |
|---|---|
| System metrics (CPU, RAM, disk) | That's Grafana/Uptime Kuma, not homeport |
| Widgets (weather, calendar, RSS) | Feature creep — other tools do this better |
| Public access / sharing | Internal tool, no use case |
| Mobile app | Browser is enough, responsive UI |
| Plugin system | Complexity without concrete need |
| Auto-accept in discovery | Decision stays with the user |
