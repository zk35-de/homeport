# homeport – Issue #79: i18n / Multi-Language Support

Implementiere i18n-Support für homeport. Alle Änderungen im Repo `/home/debian/new_pro/homeport`.

## Architektur-Übersicht

```
assets/i18n/de.json          ← Deutsche Strings (Pflicht, Fallback)
assets/i18n/en.json          ← Englische Strings
internal/i18n/i18n.go        ← Translator-Paket
```

**Design-Prinzip**: `T func(string) string` als Feld in jedem Template-Data-Struct.
Kein globaler State. Kein FuncMap-Hack. Erweiterbar: neue Sprache = neue JSON-Datei.

---

## Schritt 1: `internal/i18n/i18n.go`

Neue Datei:

```go
package i18n

import (
	"embed"
	"encoding/json"
	"log"
	"strings"
)

// translations[lang][key] = value
var translations = map[string]map[string]string{}

// Load lädt alle JSON-Dateien aus assets/i18n/ in den Speicher.
// Muss einmal beim Start aufgerufen werden (in InitTemplates oder main).
func Load(fs embed.FS) {
	entries, err := fs.ReadDir("i18n")
	if err != nil {
		log.Printf("i18n: cannot read i18n dir: %v", err)
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		lang := strings.TrimSuffix(e.Name(), ".json")
		data, err := fs.ReadFile("i18n/" + e.Name())
		if err != nil {
			log.Printf("i18n: cannot read %s: %v", e.Name(), err)
			continue
		}
		var m map[string]string
		if err := json.Unmarshal(data, &m); err != nil {
			log.Printf("i18n: cannot parse %s: %v", e.Name(), err)
			continue
		}
		translations[lang] = m
		log.Printf("i18n: loaded %d strings for lang=%s", len(m), lang)
	}
}

// T gibt einen Translator zurück: func(key) string.
// Unbekannte Keys geben den Key selbst zurück (nie leer, nie Crash).
// Unbekannte Sprachen fallen auf "de" zurück.
func T(lang string) func(string) string {
	m, ok := translations[lang]
	if !ok {
		m = translations["de"]
	}
	fallback := translations["de"]
	return func(key string) string {
		if v, ok := m[key]; ok {
			return v
		}
		if v, ok := fallback[key]; ok {
			return v
		}
		return key // Key als Notfall-Fallback – nie panic
	}
}

// SupportedLanguages gibt alle geladenen Sprachcodes zurück.
func SupportedLanguages() []string {
	langs := make([]string, 0, len(translations))
	for k := range translations {
		langs = append(langs, k)
	}
	return langs
}
```

---

## Schritt 2: `assets/i18n/de.json`

Neue Datei. Alle deutschen UI-Strings aus den Templates extrahiert:

```json
{
  "nav.manage": "Verwalten",
  "nav.logout": "Abmelden",
  "nav.analytics": "Analytics",

  "login.title": "Anmelden",
  "login.profile": "Profil",
  "login.password": "Passwort",
  "login.submit": "Anmelden",
  "login.select_profile": "Profil wählen…",
  "login.error.invalid": "Falsches Profil oder Passwort.",
  "login.error.ratelimit": "Zu viele Versuche. Bitte warte einen Moment.",

  "manage.title": "Verwalten",
  "manage.services": "Dienste",
  "manage.add_service": "Dienst hinzufügen",
  "manage.add_category": "Kategorie hinzufügen",
  "manage.add_widget": "Widget hinzufügen",
  "manage.save": "Speichern",
  "manage.cancel": "Abbrechen",
  "manage.delete": "Löschen",
  "manage.edit": "Bearbeiten",
  "manage.name": "Name",
  "manage.url": "URL",
  "manage.icon": "Icon",
  "manage.category": "Kategorie",
  "manage.profile": "Profil",
  "manage.profiles": "Profile",
  "manage.add_profile": "Profil hinzufügen",
  "manage.widget_type": "Widget-Typ",
  "manage.settings": "Einstellungen",
  "manage.theme": "Theme",
  "manage.accent_color": "Akzentfarbe",
  "manage.language": "Sprache",
  "manage.backup": "Backup",
  "manage.restore": "Wiederherstellen",
  "manage.discovery": "Discovery",
  "manage.discovery.sources": "Quellen",
  "manage.discovery.inbox": "Eingang",
  "manage.discovery.add": "Hinzufügen",
  "manage.discovery.ignore": "Ignorieren",
  "manage.auth": "Passwörter",
  "manage.auth.set": "Passwort setzen",
  "manage.auth.remove": "Passwort entfernen",
  "manage.pages": "Seiten",
  "manage.add_page": "Seite hinzufügen",
  "manage.page.title": "Titel",
  "manage.page.url": "URL",
  "manage.page.icon": "Icon",

  "widget.bookmarks.empty": "Noch keine Lesezeichen.",
  "widget.bookmarks.delete": "Löschen",
  "widget.bookmarks.name_placeholder": "Name",
  "widget.bookmarks.url_placeholder": "https://…",
  "widget.notes.placeholder": "Notizen…",
  "widget.todo.add_placeholder": "Neue Aufgabe…",
  "widget.todo.add": "Hinzufügen",
  "widget.todo.delete": "Löschen",

  "analytics.title": "Analytics",
  "analytics.filter.profile": "Profil:",
  "analytics.filter.all": "Alle",
  "analytics.back": "← Manage",
  "analytics.table.rank": "#",
  "analytics.table.service": "Dienst",
  "analytics.table.profile": "Profil",
  "analytics.table.clicks": "Klicks",
  "analytics.table.last_clicked": "Zuletzt",
  "analytics.empty": "Noch keine Klicks erfasst.",

  "404.title": "Seite nicht gefunden.",
  "404.back": "← Zurück zur Startseite",

  "pref.language.de": "Deutsch",
  "pref.language.en": "English",

  "common.loading": "Lädt…",
  "common.error": "Fehler",
  "common.confirm_delete": "Wirklich löschen?"
}
```

---

## Schritt 3: `assets/i18n/en.json`

```json
{
  "nav.manage": "Manage",
  "nav.logout": "Sign out",
  "nav.analytics": "Analytics",

  "login.title": "Sign in",
  "login.profile": "Profile",
  "login.password": "Password",
  "login.submit": "Sign in",
  "login.select_profile": "Select profile…",
  "login.error.invalid": "Wrong profile or password.",
  "login.error.ratelimit": "Too many attempts. Please wait a moment.",

  "manage.title": "Manage",
  "manage.services": "Services",
  "manage.add_service": "Add service",
  "manage.add_category": "Add category",
  "manage.add_widget": "Add widget",
  "manage.save": "Save",
  "manage.cancel": "Cancel",
  "manage.delete": "Delete",
  "manage.edit": "Edit",
  "manage.name": "Name",
  "manage.url": "URL",
  "manage.icon": "Icon",
  "manage.category": "Category",
  "manage.profile": "Profile",
  "manage.profiles": "Profiles",
  "manage.add_profile": "Add profile",
  "manage.widget_type": "Widget type",
  "manage.settings": "Settings",
  "manage.theme": "Theme",
  "manage.accent_color": "Accent color",
  "manage.language": "Language",
  "manage.backup": "Backup",
  "manage.restore": "Restore",
  "manage.discovery": "Discovery",
  "manage.discovery.sources": "Sources",
  "manage.discovery.inbox": "Inbox",
  "manage.discovery.add": "Add",
  "manage.discovery.ignore": "Ignore",
  "manage.auth": "Passwords",
  "manage.auth.set": "Set password",
  "manage.auth.remove": "Remove password",
  "manage.pages": "Pages",
  "manage.add_page": "Add page",
  "manage.page.title": "Title",
  "manage.page.url": "URL",
  "manage.page.icon": "Icon",

  "widget.bookmarks.empty": "No bookmarks yet.",
  "widget.bookmarks.delete": "Delete",
  "widget.bookmarks.name_placeholder": "Name",
  "widget.bookmarks.url_placeholder": "https://…",
  "widget.notes.placeholder": "Notes…",
  "widget.todo.add_placeholder": "New task…",
  "widget.todo.add": "Add",
  "widget.todo.delete": "Delete",

  "analytics.title": "Analytics",
  "analytics.filter.profile": "Profile:",
  "analytics.filter.all": "All",
  "analytics.back": "← Manage",
  "analytics.table.rank": "#",
  "analytics.table.service": "Service",
  "analytics.table.profile": "Profile",
  "analytics.table.clicks": "Clicks",
  "analytics.table.last_clicked": "Last clicked",
  "analytics.empty": "No clicks recorded yet.",

  "404.title": "Page not found.",
  "404.back": "← Back to home",

  "pref.language.de": "Deutsch",
  "pref.language.en": "English",

  "common.loading": "Loading…",
  "common.error": "Error",
  "common.confirm_delete": "Really delete?"
}
```

---

## Schritt 4: `assets/assets.go` – embed i18n

Die Datei `assets/assets.go` muss die neue `i18n/`-Directory embedden. Lies die Datei zuerst.
Füge dort hinzu, dass `//go:embed` auch `i18n/*.json` einschließt, und dass das FS-Objekt auch für `i18n.Load()` zugänglich ist.

Typisch sieht das so aus – passe es an was bereits dort steht:

```go
//go:embed templates static i18n
var Assets embed.FS
```

---

## Schritt 5: `internal/api/index.go` – i18n initialisieren & Data-Structs

### 5a: Import hinzufügen
```go
"git.zk35.de/secalpha/homeport/internal/i18n"
```

### 5b: In `InitTemplates()` am Anfang ergänzen:
```go
i18n.Load(fs)
```

### 5c: `IndexData` Struct – T-Feld hinzufügen:
```go
type IndexData struct {
    Categories   []db.Category
    Widgets      []db.Widget
    Pages        []db.Page
    Profile      string
    ProfileName  string
    SearchAction string
    Prefs        *db.UserPreferences
    Profiles     []db.Profile
    T            func(string) string  // NEU
}
```

### 5d: In `HandleIndex()` beim Befüllen von `data`:
```go
data := IndexData{
    // ... bestehende Felder ...
    T: i18n.T(prefs.Language),
}
```

### 5e: `Handle404` – Language aus Accept-Language Header oder "de":
Da der 404-Handler keinen User-Kontext hat, verwende "de" als Default:
```go
t := i18n.T("de")
```
Und ersetze die hardcodierten deutschen Strings durch `t("404.title")` und `t("404.back")`.
Baue den Body-String mit `fmt.Sprintf` oder string concatenation mit den t()-Ergebnissen.

---

## Schritt 6: `internal/api/manage.go` – ManageData

### 6a: ManageData Struct – lies die Datei zuerst, ergänze T-Feld:
```go
type ManageData struct {
    // ... bestehende Felder ...
    T func(string) string  // NEU
}
```

### 6b: Im Handler beim Befüllen:
```go
T: i18n.T(prefs.Language),
```

### 6c: Alle anderen ManageData-Initialisierungen (z.B. für HTMX-Partials) ebenfalls anpassen.

---

## Schritt 7: `internal/api/analytics.go` – AnalyticsData

```go
type AnalyticsData struct {
    Stats    []db.ClickStat
    Profile  string
    Profiles []db.Profile
    Prefs    *db.UserPreferences
    T        func(string) string  // NEU
}
```

Im Handler: `T: i18n.T(prefs.Language),`

---

## Schritt 8: `internal/api/auth.go` – Login

### 8a: LoginData struct erstellen (bisher anonym – bleibt anonym ist ok, aber T hinzufügen):
```go
data := struct {
    Error    string
    Profiles []db.Profile
    T        func(string) string
}{
    Error: errMsg,
    T:     i18n.T("de"), // Login: kein User-Kontext → immer "de" (oder Accept-Language)
}
```

### 8b: Die `renderLogin`-Funktion erhält einen `lang string`-Parameter:
```go
func renderLogin(w http.ResponseWriter, errMsg string, lang string) {
```
Alle Aufrufe von `renderLogin(w, "...")` müssen angepasst werden auf `renderLogin(w, "...", "de")`.

---

## Schritt 9: Widget-Partials (standalone render)

Die Partials `widget_todo`, `widget_bookmarks` werden standalone via `ExecuteTemplate(w, "widget_todo", widget)` gerendert – hier ist `db.Widget` das Data-Objekt, das kein `T` hat.

Lösung: Wrapper-Struct in `internal/api/widget_render.go` (neue Datei):

```go
package api

import (
    "git.zk35.de/secalpha/homeport/internal/db"
    "git.zk35.de/secalpha/homeport/internal/i18n"
)

// WidgetRenderData wraps a Widget with a translator for standalone partial renders.
type WidgetRenderData struct {
    db.Widget
    T func(string) string
}

func newWidgetRender(w db.Widget, lang string) WidgetRenderData {
    return WidgetRenderData{Widget: w, T: i18n.T(lang)}
}
```

Passe `todo.go` und `bookmarks.go` an:
- Hole die Sprache aus `db.GetUserPreferences(widget.Profile)` (kann nil sein → "de")
- Übergib `newWidgetRender(widget, lang)` statt `widget` an `ExecuteTemplate`

---

## Schritt 10: Templates anpassen

**Wichtig**: Lies jeden Template vor dem Bearbeiten. Ersetze NUR hardcodierte Strings. Lass alle `{{.Foo}}`, `{{range}}`, `{{if}}` etc. unverändert.

### `assets/templates/base.html`
Ersetze:
- "Abmelden" → `{{.T "nav.logout"}}`
- "Verwalten" / "Manage"-Link → `{{.T "nav.manage"}}`
- Weitere hardcodierte UI-Strings

### `assets/templates/login.html`
- Titel, Labels, Button → `.T`-Aufrufe
- Fehlermeldungen kommen bereits als `{{.Error}}` – beibehalten
  (Die Strings "Falsches Profil..." werden jetzt in auth.go per `t("login.error.invalid")` gesetzt)
- Den Error-String in `renderLogin` und `HandleLogin` ebenfalls durch `t(...)` ersetzen

### `assets/templates/manage.html`
- Alle Buttons, Labels, Überschriften, Placeholders → `.T`-Aufrufe
- Lies die Datei vollständig, geh systematisch durch

### `assets/templates/analytics.html`
- Tabellenheader, Leerstate-Text → `.T`-Aufrufe

### `assets/templates/partials/widget_bookmarks.html`
- `title="Löschen"` → `{{.T "widget.bookmarks.delete"}}`
- `placeholder="Name"` → `{{.T "widget.bookmarks.name_placeholder"}}`
- `placeholder="https://…"` → `{{.T "widget.bookmarks.url_placeholder"}}`
- Leerstate-Text → `{{.T "widget.bookmarks.empty"}}`

### `assets/templates/partials/widget_todo.html`
- Lies und ersetze hardcodierte Strings

### `assets/templates/partials/widget_notes.html`
- `placeholder="Notizen…"` → `{{.T "widget.notes.placeholder"}}`

### `assets/templates/index.html`
- Prüfe auf hardcodierte UI-Strings (Nav, Buttons)

---

## Schritt 11: Language-Selector in User-Preferences

In `assets/templates/manage.html` im Bereich User-Preferences / Settings: Füge einen Sprachauswahl-Dropdown hinzu (falls noch nicht vorhanden):

```html
<div class="pref-row">
  <label>{{.T "manage.language"}}</label>
  <select name="language" hx-post="/api/preferences/..." hx-trigger="change">
    <option value="de" {{if eq .Prefs.Language "de"}}selected{{end}}>{{.T "pref.language.de"}}</option>
    <option value="en" {{if eq .Prefs.Language "en"}}selected{{end}}>{{.T "pref.language.en"}}</option>
  </select>
</div>
```

Schaue dir die bestehende Preferences-API an (`preferences_api.go`) – `language` ist bereits in `SetUserPreferences` verarbeitet, also braucht es keine Backend-Änderungen.

---

## Schritt 12: Build & Test

```bash
cd /home/debian/new_pro/homeport
go build ./...
go test ./...
```

Fehler beheben. Dann:

```bash
git add -A
git commit -m "feat: i18n multi-language support (DE/EN), closes #79

- internal/i18n: embed-based JSON translation loader, T(lang) func
- assets/i18n/de.json + en.json with all UI strings
- T func(string) string injected into all *Data structs
- WidgetRenderData wrapper for standalone partial renders
- Language selector in User Preferences (field was already in DB)
- Future languages: drop new JSON file in assets/i18n/"
```

---

## Wichtige Constraints

1. **Kein globaler State** für die aktive Sprache – immer per-Request
2. **Fallback immer auf "de"** – nie rohe Keys anzeigen
3. **Neues JSON = neue Sprache** – kein Code-Change nötig für zusätzliche Sprachen
4. **Lies jede Datei bevor du sie bearbeitest**
5. **`go build ./...` muss am Ende fehlerfrei durchlaufen**
6. **`go test ./...` muss am Ende grün sein**
