# Dashy vs. homeport – Feature-Vergleich

Stand: 2026-03-23

## Übersicht

| Feature | Dashy | homeport |
|---|---|---|
| **Lizenz** | MIT | MIT |
| **Stack** | Vue.js + Node.js | Go + HTMX |
| **Config** | YAML-Datei + Web-Editor | SQLite + Web-UI |
| **Multi-Profile** | ✗ (Single-Config) | ✓ (z.B. Markus, Andrea, …) |
| **Multi-Page / Tabs** | ✓ | ✓ |
| **Layouts** | tiles, list, workspace, startpage | tiles, list, icons |
| **Service Status** | ✓ | ✓ |
| **Klick-Tracking** | ✗ | ✓ |
| **Smart Sort (Usage)** | ✗ | ✓ |
| **Drag & Drop** | ✓ | ✓ |
| **Themes** | ~30 vordefinierte Themes | dark/light + accent color + custom CSS |
| **Custom CSS** | ✓ | ✓ |
| **PWA** | ✓ | ✓ |
| **Suche** | ✓ + Tags + Bangs | ✓ + Spotlight + Bangs |
| **Auth** | ✓ Multi-User + SSO/Keycloak | ✓ Multi-User (kein SSO) |
| **Auto-Discovery** | ✗ | ✓ (Podman, Docker, NPM, Traefik) |
| **Widgets gesamt** | ~60+ | ~10 |
| **iCal / Kalender** | ✗ | ✓ |
| **Weather** | ✓ | ✓ |
| **RSS** | ✓ | ✓ |
| **Todos** | ✗ | ✓ |
| **Notes** | ✗ | ✓ |
| **Bookmarks** | ✗ | ✓ |
| **GitHub Widget** | ✓ | ✓ |
| **System Metrics** | ✓ (umfangreich) | ✗ (offen, Issue #72) |
| **Proxmox Widget** | ✓ | ✗ |
| **Pi-Hole / AdGuard / etc.** | ✓ | ✗ |
| **Router Widget** | ✗ | ✓ (Speedport, FritzBox) |
| **CalDAV** | ✗ | ✓ |
| **Cloud Backup** | ✓ (encrypted) | ✗ |
| **Mehrsprachig** | ✓ (10+ Sprachen) | 🚧 in Entwicklung (Issue #79) |
| **Mobile** | ✓ | ✓ |
| **Binary-Größe** | groß (Node.js) | klein (single Go binary) |
| **Public release** | seit Jahren etabliert | in Entwicklung |

## Fazit

Dashy ist erheblich feature-reicher bei Widgets und bietet SSO sowie Cloud-Backup.

homeport hat Vorteile bei:
- **Multi-Profil mit unterschiedlichen Sichtbarkeiten** – bei Dashy nicht vorhanden
- **Auto-Discovery** (Podman, Docker, NPM, Traefik)
- **Lokal-first / Privacy** – kein Cloud-Zwang, kein externer Dienst
- **Footprint** – single Go binary, kein Node.js
- **Router-Widget** (Speedport AES-Entschlüsselung, FritzBox TR-064)
- **CalDAV-Widget**

Der eigentliche USP von homeport – verschiedene Profile für verschiedene Familienmitglieder mit unterschiedlicher Service-Sichtbarkeit – existiert bei Dashy gar nicht.
