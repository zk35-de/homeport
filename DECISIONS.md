# homeport – Decisions

Quelle der Wahrheit für Vision und Design-Entscheidungen.

---

## Vision

> **Die komfortabelste selbstgehostete Startpage – Multi-user mit getrennten Bereichen, ohne Config-Dateien anzufassen, mit Spotlight-Suche.**

### Warum homeport existiert

Bestehende selbstgehostete Startpages haben mindestens eines dieser Probleme:
1. **Config-Dateien** – Bookmarks in YAML/JSON editieren ist kein UX, das ist Systemadministration
2. **Kein echtes Multi-user** – alle sehen dasselbe, oder Trennung ist nur kosmetisch
3. **Supply Chain** – unnötige Dependencies, aufgeblähte Images, keine Digest-Pins
4. **Feature-Creep** – Monitoring, Widgets, Wetter-API – alles außer dem was eine Startpage sein soll

homeport löst diese vier Probleme. Nicht mehr, nicht weniger.

---

## Nutzer

**Markus** – IT/NetSec-Links, interne Tools, häufige Nutzung diverser Kategorien
**Andrea** – andere Interessensgebiete (Pinterest etc.), will Markus' Links nicht sehen

Beide nutzen dasselbe homeport, sehen aber ihren eigenen Bookmark-Bereich.

---

## Killer-Features (Alleinstellungsmerkmale)

### 1. Multi-user mit getrennten Bookmark-Spaces
Jeder User hat seinen eigenen Bereich. Keine gemeinsame Ansicht die man wegklicken muss.
Nicht "Favoriten pro User" auf einem shared Pool – komplett getrennte Spaces.

### 2. Spotlight-Suche für lokale Bookmarks
Tastaturkürzel → Suchfeld → tippen → direkt zum Link.
Kein Scrollen, kein Kategorien-Durchsuchen. Funktioniert nur auf eigenen Bookmarks, nicht im Web.

### 3. Sort by Usage innerhalb Kategorie
Häufig genutzte Links wandern nach oben. Automatisch, ohne manuelles Sortieren.
Pro User – Markus' Nutzungsverhalten beeinflusst Andreas Reihenfolge nicht.

### 4. Zero Config-File-Editing
Alles über die UI. Kein YAML, kein JSON, kein Neustart nach Änderung.
Onboarding = URL öffnen, Account anlegen, loslegen.

---

## Status Check

Bookmarks können einen optionalen Status-Check haben (ist der Dienst erreichbar?).
**Kein Monitoring.** Kein Alerting, keine Metriken, keine History, keine Graphen.
Der Unterschied: eine Startpage zeigt ob ein Link gerade funktioniert – sie ersetzt nicht Uptime Kuma.

---

## Supply Chain (nicht verhandelbar)

- Minimale Dependencies – jede neue Dependency braucht eine Begründung
- Base-Image mit Digest-Pin im Containerfile
- govulncheck im CI
- Kein CDN-Load für externe Assets – alles self-contained

---

## Bewusste Nicht-Entscheidungen

| Was | Warum nicht |
|---|---|
| Widgets (Wetter, Kalender, RSS) | Feature-Creep, andere Tools machen das besser |
| Monitoring / Metriken | Das ist Uptime Kuma, nicht homeport |
| Öffentlicher Zugang / Sharing | Internes Tool, kein Use-Case |
| Mobile App | Browser reicht, responsive genug |
| Plugin-System | Komplexität ohne konkreten Bedarf |
| Shared Bookmarks zwischen Usern | Verleitet zu "gemeinsamen" Bereichen – der Kern ist Trennung |
