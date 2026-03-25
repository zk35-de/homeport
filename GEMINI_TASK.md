# homeport – Cleanup Sprint: Issues #91, #92, #93, #94, #95, #96

Implementiere alle sechs Cleanups in separaten Commits. Alle Änderungen im Repo `/home/debian/new_pro/homeport`.

**Nach jedem Issue:** `go build -o homeport ./cmd/homeport/ && go test ./...` – beide müssen grün sein, sonst nicht committen.

---

## Issue #91 – Totes `background`-Feld entfernen

Das Feld `Background string` in `UserPreferences` wird zwar in der DB gespeichert, aber von keinem Template gelesen. Es wurde durch `background_mode` (Migration 6) ersetzt und vergessen.

### `internal/db/models.go`

Lies die Datei. Entferne aus dem `UserPreferences`-Struct:
```go
Background     string `json:"background"`
```

### `internal/db/profiles.go`

Lies die Datei. Es gibt eine `GetUserPreferences()`-Funktion mit einem SELECT-Query und einem `row.Scan(...)`.

1. Im SELECT-Query: `background,` entfernen
2. Im `row.Scan(...)`: `&p.Background,` entfernen
3. In der Defaults-Initialisierung: falls `Background: "aurora"` vorkommt, entfernen
4. In `SetUserPreferences()`: im INSERT/UPDATE-Query `background` aus der Spaltenliste und aus den Values entfernen, sowie `prefs.Background` aus den Parametern entfernen

### `internal/api/preferences_api.go`

Lies die Datei. Entferne den Block:
```go
if v, ok := patch["background"]; ok {
    current.Background = v
}
```

### Build + Test + Commit

```bash
go build -o homeport ./cmd/homeport/ && go test ./...
git add internal/db/models.go internal/db/profiles.go internal/api/preferences_api.go
git commit -m "cleanup: remove dead background field from UserPreferences (#91)

Field was superseded by background_mode in migration 6 but never removed.
DB column stays (SQLite has no DROP COLUMN before 3.35) but is no longer
read or written by Go code."
```

---

## Issue #92 – Aurora/Tageszeit-Hintergrund entfernen

Aurora und Tageszeit-Hintergrund sind reine Dekoration. Danach gibt es nur noch einen schlichten dunklen/hellen Hintergrund per CSS.

### `assets/templates/base.html`

Lies die Datei. Entferne:
1. `<link rel="stylesheet" href="/static/css/prism-aurora.css">` (die Zeile mit aurora.css)
2. Die `<div class="aurora" id="bg-aurora">...</div>` Sektion (alle Zeilen des Aurora-Divs)
3. Die `<div class="bg-time" id="bg-time" ...></div>` Zeile
4. Die `<div class="bg-image" id="bg-image" ...></div>` Zeile
5. Den `<script>`-Block der `data-bg` auswertet und `setBgMode()` aufruft (der Block direkt nach den bg-Divs)
6. Das `data-bg="..."` Attribut aus dem `<body>`-Tag

### `assets/templates/manage.html`

Lies die Datei. Entferne im Appearance-Abschnitt:
1. Den Label "Hintergrund" / "Background" mit den drei Buttons (Aurora, Zeit, Keine)
2. Die `setBgMode()`-Funktion im JS-Bereich
3. Den Aufruf `savePrefs({background_mode: mode})` (der in `setBgMode` steckt – fällt automatisch weg)

### `internal/db/models.go`

Entferne aus `UserPreferences`:
```go
BackgroundMode string `json:"background_mode"`
```

### `internal/db/profiles.go`

Lies die Datei:
1. Im SELECT-Query: `COALESCE(background_mode,'aurora')` entfernen
2. Im `row.Scan(...)`: `&p.BackgroundMode` entfernen
3. In der Defaults-Initialisierung: `BackgroundMode: "aurora"` entfernen
4. In `SetUserPreferences()`: `background_mode` aus INSERT/UPDATE-Query entfernen, `prefs.BackgroundMode` aus Parametern entfernen

### `internal/api/preferences_api.go`

Entferne den Block:
```go
if v, ok := patch["background_mode"]; ok {
    current.BackgroundMode = v
}
```

### `assets/static/css/prism-aurora.css`

Prüfe ob diese Datei existiert: `ls assets/static/css/prism-aurora.css`
Falls ja: löschen.

### Build + Test + Commit

```bash
go build -o homeport ./cmd/homeport/ && go test ./...
git add -A
git commit -m "cleanup: remove aurora/time background modes (#92)

Pure decoration with no functional value. Removes prism-aurora.css,
background divs, JS mode switcher, and background_mode pref field.
Plain CSS background remains."
```

---

## Issue #93 – clone-andrea durch generisches Profil-Klonen ersetzen

`POST /manage/clone-andrea` und `CloneToAndrea()` sind hardcodiert auf Profilnamen "markus" und "andrea". Dazu kommt hardcodiertes "andrea" in `analytics.go`.

### `internal/db/analytics.go`

Lies die Datei. Finde die Funktion die "andrea" hartcodiert verwendet (ca. Zeile 54ff). Diese Funktion heißt vermutlich `CloneToAndrea` oder ähnlich und enthält:
```sql
WHERE profile = 'andrea'
INSERT INTO visibility (service_id, profile) VALUES (?, 'andrea')
```

Ersetze die gesamte Funktion durch eine generische Version:

```go
// CloneServicesToProfile copies all services visible to srcProfile into dstProfile.
// Services in categories with color 'cyan' are excluded (admin-only convention).
// Returns counts of added and skipped services.
func CloneServicesToProfile(srcProfile, dstProfile string) (int, int, error) {
	tx, err := DB.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	rows, err := tx.Query(`
		SELECT s.id FROM services s
		JOIN visibility v ON v.service_id = s.id
		JOIN categories c ON c.id = s.category_id
		WHERE v.profile = ? AND c.color != 'cyan'`, srcProfile)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()

	added, skipped := 0, 0
	for rows.Next() {
		var serviceID int
		if err := rows.Scan(&serviceID); err != nil {
			return 0, 0, err
		}
		var exists int
		err := tx.QueryRow(`SELECT COUNT(*) FROM visibility WHERE service_id = ? AND profile = ?`, serviceID, dstProfile).Scan(&exists)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to check visibility for service %d and profile %s: %w", serviceID, dstProfile, err)
		}
		if exists > 0 {
			skipped++
			continue
		}
		_, err = tx.Exec(`INSERT INTO visibility (service_id, profile) VALUES (?, ?)`, serviceID, dstProfile)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to insert visibility for service %d and profile %s: %w", serviceID, dstProfile, err)
		}
		added++
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return added, skipped, nil
}
```

Prüfe welche Imports die Datei bereits hat. Falls `fmt` noch nicht importiert ist, ergänzen.

### `internal/api/manage.go`

Lies die Datei. Finde `HandleCloneToAndrea`. Ersetze die gesamte Funktion durch:

```go
// HandleCloneProfile POST /manage/profile/{slug}/clone
// Clones all services from the given profile to a new profile specified in the form.
func HandleCloneProfile(w http.ResponseWriter, r *http.Request) {
	srcSlug := chi.URLParam(r, "slug")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	dstName := strings.TrimSpace(r.FormValue("name"))
	if dstName == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	dstSlug := strings.ToLower(strings.ReplaceAll(dstName, " ", "-"))

	// Create destination profile if it doesn't exist
	profiles, _ := db.GetProfiles()
	exists := false
	for _, p := range profiles {
		if p.Slug == dstSlug {
			exists = true
			break
		}
	}
	if !exists {
		if err := db.AddProfile(dstSlug, dstName); err != nil {
			log.Printf("HandleCloneProfile: AddProfile error: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	added, skipped, err := db.CloneServicesToProfile(srcSlug, dstSlug)
	if err != nil {
		log.Printf("HandleCloneProfile: CloneServicesToProfile error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	log.Printf("CloneProfile %s→%s: added=%d skipped=%d", srcSlug, dstSlug, added, skipped)
	HandleManage(w, r)
}
```

Prüfe ob `strings` bereits importiert ist; falls nicht, ergänzen.

### `cmd/homeport/main.go`

Lies die Datei. Ersetze:
```go
r.Post("/clone-andrea", api.HandleCloneToAndrea)
```
durch:
```go
r.Post("/profile/{slug}/clone", api.HandleCloneProfile)
```

### `assets/templates/manage.html`

Lies die Datei. Finde den Bereich mit dem "clone-andrea"-Formular oder Button. Ersetze ihn durch ein generisches Klonen-UI. Suche nach `clone-andrea` oder `CloneToAndrea` oder einem Button/Form der darauf zeigt.

Ersetze das bestehende Formular durch:
```html
<form hx-post="/manage/profile/{{.DefaultProfile}}/clone" hx-swap="outerHTML" hx-target="body" style="display:flex;gap:0.5rem;align-items:center;margin-top:0.5rem">
  <input type="text" name="name" placeholder="{{.T "manage.clone_target_name"}}" class="form-input" required>
  <button type="submit" class="btn-sm">{{.T "manage.clone_profile"}}</button>
</form>
```

Falls `.DefaultProfile` nicht im ManageData-Struct vorhanden ist: lies `manage.go`, finde `ManageData` und ergänze:
```go
DefaultProfile string
```
Und beim Befüllen: hole es via `db.GetDefaultProfile()`.

### i18n – neue Strings

Lies `assets/i18n/de.json` und `assets/i18n/en.json`. Ergänze:

**de.json:**
```json
"manage.clone_profile": "Profil klonen",
"manage.clone_target_name": "Name des neuen Profils"
```

**en.json:**
```json
"manage.clone_profile": "Clone profile",
"manage.clone_target_name": "New profile name"
```

### Build + Test + Commit

```bash
go build -o homeport ./cmd/homeport/ && go test ./...
git add -A
git commit -m "feat: replace hardcoded clone-andrea with generic profile clone (#93)

- db: CloneServicesToProfile(src, dst string) replaces CloneToAndrea()
- api: HandleCloneProfile replaces HandleCloneToAndrea
- route: POST /manage/profile/{slug}/clone
- analytics: no more hardcoded 'andrea' profile name
- i18n: new strings for clone UI"
```

---

## Issue #94 – Category-Layout-Feld entfernen (nie implementiert)

Das `Layout`-Feld in `Category` und `UserPreferences` setzt CSS-Klassen wie `grid-tiles`, `grid-list`, `grid-icons`, die in der CSS-Datei nicht existieren. Feature wurde nie fertiggestellt. Entfernen ist sauberer als halbfertig lassen.

### `internal/db/models.go`

Lies die Datei. Entferne:
1. `Layout string` aus dem `Category`-Struct
2. `Layout string` aus dem `UserPreferences`-Struct (falls vorhanden – prüfen)

### `internal/db/profiles.go` (UserPreferences Layout)

Falls `layout` in `GetUserPreferences()` oder `SetUserPreferences()` vorkommt: entfernen (SELECT, Scan, INSERT, UPDATE).

### `internal/db/` – Category-Queries

Lies die Datei(en) die `categories`-DB-Operationen enthalten (wahrscheinlich `services.go` oder `categories.go` oder `db.go`). Suche nach Funktionen die Kategorien anlegen oder lesen. Entferne `layout` aus allen Category-INSERT/UPDATE/SELECT-Queries und den zugehörigen Scan/Parameter-Calls.

### `assets/templates/index.html`

Lies die Datei. Finde `class="grid-{{.Layout}}"`. Ersetze durch einfaches `class="grid"` (oder das passende bestehende Grid-CSS, z.B. `class="category-grid"`). Schau was die CSS-Datei `prism-base.css` als Klasse für das Service-Grid verwendet und nutze die korrekte Klasse.

### `assets/templates/manage.html`

Lies die Datei. Entferne im Kategorie-Formular (Add + Edit):
- Den `<div class="form-group">` Block mit `<label>Layout</label>` und dem `<select name="layout">`
- Ebenso im Edit-Formular für Kategorien

### `internal/api/manage.go`

Lies die Datei. Entferne in `HandleAddCategory` und `HandleUpdateCategory` die Zeilen die `layout` aus dem Formular lesen und an DB-Funktionen übergeben.

### `internal/api/preferences_api.go`

Falls `layout` im Preferences-Patch-Handler vorkommt: entfernen.

### Build + Test + Commit

```bash
go build -o homeport ./cmd/homeport/ && go test ./...
git add -A
git commit -m "cleanup: remove unimplemented category layout field (#94)

CSS classes grid-tiles/grid-list/grid-icons never existed. Field stored
in DB but had no visual effect. Removes layout from Category model,
UserPreferences model, all DB queries, and manage UI forms."
```

---

## Issue #95 – /api/updates SSE-Endpoint entfernen

`GET /api/updates` ist ein Bearer-geschützter SSE-Endpoint der vom Frontend nie genutzt wird. Das Frontend verwendet ausschließlich `GET /status/stream`.

### Analyse zuerst

```bash
grep -rn "DefaultHub\|HandleUpdates\|Broadcast" /home/debian/new_pro/homeport/internal/ --include="*.go"
```

Prüfe ob `DefaultHub.Broadcast` außer in `status.go` noch woanders aufgerufen wird.

### `internal/api/updates.go`

Lies die Datei. Entscheide basierend auf der Analyse:

**Falls `DefaultHub.Broadcast` nur in `status.go` und `updates.go` selbst vorkommt:**
- Lösche `updates.go` komplett
- Entferne in `status.go` den Import und den `DefaultHub.Broadcast(...)`-Aufruf

**Falls `DefaultHub.Broadcast` noch woanders genutzt wird:**
- Nur die `HandleUpdates`-Funktion und `UpdateHub.HandleUpdates`-Methode entfernen
- `DefaultHub` und `Broadcast` behalten

### `cmd/homeport/main.go`

Lies die Datei. Entferne:
```go
// SSE Live Updates
r.Get("/updates", api.DefaultHub.HandleUpdates)
```

Prüfe danach ob die Bearer-Auth-Gruppe (`r.Group(func(r chi.Router) { r.Use(api.AuthMiddleware(...)) ... })`) leer ist. Falls ja: die gesamte leere Gruppe entfernen. Hinweis: `AuthMiddleware` selbst noch NICHT entfernen – das kommt in Issue #97 nach #90.

### Build + Test + Commit

```bash
go build -o homeport ./cmd/homeport/ && go test ./...
git add -A
git commit -m "cleanup: remove unused /api/updates SSE endpoint (#95)

Frontend uses /status/stream exclusively. /api/updates was behind
Bearer auth and never consumed by any browser code."
```

---

## Issue #96 – Doppelten Search-Engine-Endpoint entfernen

Die Suchmaschine kann über zwei Wege gesetzt werden: `POST /manage/settings/search` und `PATCH /api/user/preferences`. Ersterer wird entfernt.

### `internal/db/` – SetSearchEngine / GetAllSearchEngines

Lies die DB-Dateien (wahrscheinlich `profiles.go` oder eine separate Datei). Entferne:
- `func SetSearchEngine(profile, engine string) error`
- `func GetAllSearchEngines() map[string]string`

### `internal/api/manage.go`

Lies die Datei:
1. Entferne `func HandleSetSearchEngine(...)` komplett
2. In `HandleManage` und `ManageData`: entferne `SearchEngines map[string]string` und den Aufruf `db.GetAllSearchEngines()`
3. In der `ManageData`-Struct-Definition: `SearchEngines`-Feld entfernen

### `cmd/homeport/main.go`

Entferne die Route:
```go
r.Post("/settings/search", api.HandleSetSearchEngine)
```

### `assets/templates/manage.html`

Lies die Datei. Finde den Bereich mit `hx-post="/manage/settings/search"` oder einem Search-Engine-Formular das auf diese Route posted.

Ersetze es durch einen HTMX-Patch auf die Preferences-API (wie das Language-Dropdown bereits funktioniert):

```html
<select name="search_engine"
        hx-patch="/api/user/preferences"
        hx-ext="json-enc"
        hx-trigger="change"
        hx-swap="none">
  <option value="https://duckduckgo.com/" {{if eq .Prefs.SearchEngine "https://duckduckgo.com/"}}selected{{end}}>DuckDuckGo</option>
  <option value="https://www.google.com/search" {{if eq .Prefs.SearchEngine "https://www.google.com/search"}}selected{{end}}>Google</option>
  <option value="https://search.brave.com/search" {{if eq .Prefs.SearchEngine "https://search.brave.com/search"}}selected{{end}}>Brave</option>
  <option value="https://www.startpage.com/search" {{if eq .Prefs.SearchEngine "https://www.startpage.com/search"}}selected{{end}}>Startpage</option>
  <option value="https://www.bing.com/search" {{if eq .Prefs.SearchEngine "https://www.bing.com/search"}}selected{{end}}>Bing</option>
</select>
```

Passe den umliegenden HTML-Kontext an (Label etc. beibehalten, nur das Form/Action austauschen).

### Build + Test + Commit

```bash
go build -o homeport ./cmd/homeport/ && go test ./...
git add -A
git commit -m "cleanup: remove duplicate search engine endpoint (#96)

POST /manage/settings/search replaced by PATCH /api/user/preferences
with search_engine field. Removes HandleSetSearchEngine, SetSearchEngine,
GetAllSearchEngines and the dedicated route."
```

---

## Abschluss

Nach allen sechs Commits:

```bash
git push origin main
```

Falls Push rejected (remote ahead):
```bash
git pull --rebase && git push origin main
```

**Nicht in diesem Task:** Issues #90 (Widgets komplett raus + no_check) und #97 (Bearer-Auth komplett raus) – diese haben Abhängigkeiten und kommen separat.
