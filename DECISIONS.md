# homeport – Decisions

Quelle der Wahrheit für Vision und Design-Entscheidungen.

---

## Vision

> **Die komfortabelste selbstgehostete Startpage – Multi-user mit getrennten Ansichten, ohne Config-Dateien anzufassen, mit Spotlight-Suche.**

### Warum homeport existiert

Bestehende selbstgehostete Startpages haben mindestens eines dieser Probleme:
1. **Config-Dateien** – Bookmarks in YAML/JSON editieren ist kein UX, das ist Systemadministration
2. **Kein echtes Multi-user** – alle sehen dasselbe, oder Trennung ist nur kosmetisch
3. **Supply Chain** – unnötige Dependencies, aufgeblähte Images, keine Digest-Pins
4. **Feature-Creep** – Monitoring, Widgets, Wetter-API, System-Metriken – alles außer dem was eine Startpage sein soll

homeport löst diese vier Probleme. Nicht mehr, nicht weniger.

---

## Nutzer

**Markus** – IT/NetSec-Links, interne Tools, häufige Nutzung diverser Kategorien
**Andrea** – andere Interessensgebiete (Pinterest etc.), will Markus' Links nicht sehen

Beide nutzen dasselbe homeport, sehen aber ihre eigene Ansicht.

---

## Killer-Features (Alleinstellungsmerkmale)

### 1. Multi-user mit profilbasierter Visibility
Services werden einmal angelegt (admin) und können dann gezielt pro Profil sichtbar geschaltet werden.
Kein doppeltes Pflegen, keine Synchronisation – ein Service, beliebig viele Sichtbarkeiten.
Jedes Profil sieht nur was für es relevant ist. Markus' NetSec-Links erscheinen nicht bei Andrea.

### 2. Spotlight-Suche für lokale Bookmarks
Tastaturkürzel (`Ctrl+K` / `/`) → Suchfeld → tippen → direkt zum Link.
DOM-basiert, kein Server-Roundtrip. Funktioniert nur auf eigenen Bookmarks, nicht im Web.
Bang-Syntax für externe Suchen: `!g`, `!d`, `!gh`, `!yt`, `!w` u.a.

### 3. Sort by Usage innerhalb Kategorie
Click-Count pro Service pro Profil → häufig genutzte Links wandern nach oben.
Automatisch, ohne manuelles Sortieren. Pro Profil – Markus' Nutzungsverhalten beeinflusst Andreas Reihenfolge nicht.
Click-Analytics (Top-25) in `/manage/analytics` als Einblick in eigene Nutzung.

### 4. Zero Config-File-Editing
Alles über die UI. Kein YAML, kein JSON, kein Neustart nach Änderung.
Onboarding = URL öffnen, Profil anlegen, loslegen.

### 5. Auto-Discovery
homeport erkennt neue Dienste automatisch aus konfigurierten Quellen:
- **Nginx Proxy Manager** – REST API
- **Traefik** – HTTP Provider
- **Docker/Podman TCP** – Container-Labels (`homeport.name`, `homeport.url` etc.)

Gefundene Dienste landen im Discovery Inbox zur manuellen Freigabe – kein automatisches Hinzufügen.
Entscheidung bleibt beim Nutzer, Discovery nimmt nur die Tipparbeit weg.

---

## Status Check

Bookmarks können einen optionalen Status-Check haben (ist der Dienst erreichbar?).
Ergebnis: grüner/roter Glow am Service-Icon, alle 30 Sekunden aktualisiert per SSE.

**Kein System-Monitoring.** homeport zeigt nicht CPU-Last des PVE, freien Speicher der Synology
oder RAM-Verbrauch von VMs. Das machen andere Tools (Grafana, Uptime Kuma) besser.
homeport ist eine Startpage, kein Dashboard.

---

## Supply Chain (nicht verhandelbar)

- Minimale Dependencies – jede neue Dependency braucht eine Begründung
- Base-Image mit Digest-Pin im Containerfile
- `govulncheck` im CI
- Kein CDN-Load für externe Assets – htmx, sse.js lokal eingebettet, alles self-contained

---

## Deployment-Modell

homeport ist für vertrauenswürdige Netzwerke (Homelab, privates LAN) konzipiert.
Nicht direkt ins Internet exponieren. Bei Remote-Zugriff: Reverse Proxy mit TLS + `HOMEPORT_AUTH=true`.

---

## Bewusste Nicht-Entscheidungen

| Was | Warum nicht |
|---|---|
| System-Metriken (CPU, RAM, Disk) | Das ist Grafana/Uptime Kuma, nicht homeport |
| Widgets (Wetter, Kalender, RSS) | Feature-Creep, andere Tools machen das besser |
| Öffentlicher Zugang / Sharing | Internes Tool, kein Use-Case |
| Mobile App | Browser reicht, responsive genug |
| Plugin-System | Komplexität ohne konkreten Bedarf |
| Automatisches Accept in Discovery | Entscheidung bleibt beim Nutzer |
