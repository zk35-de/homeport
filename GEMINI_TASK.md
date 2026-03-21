# homeport – Issues #26, #30, #36

Implementiere drei Features in einem Commit. Alle Änderungen im Repo `/home/debian/new_pro/homeport`.

---

## #26 – Bookmarks Widget

Neues Widget-Typ `bookmarks`: Liste von Links mit Favicon, ähnlich wie das Todo-Widget.

### db.go – Ergänzungen

1. Neues Struct nach `TodoItem`:
```go
type BookmarkLink struct {
    Name string `json:"name"`
    URL  string `json:"url"`
    Icon string `json:"icon"`
}
```

2. Im `Widget` Struct nach `Todos []TodoItem` ergänzen:
```go
BookmarkLinks []BookmarkLink `json:"-"` // populated for type=bookmarks
NoteContent   string         `json:"-"` // populated for type=notes
```

3. In `populateWidgetFields()` nach dem clock-Block:
```go
if w.Type == "bookmarks" {
    var cfg struct {
        Links []BookmarkLink `json:"links"`
    }
    if err := json.Unmarshal([]byte(w.Config), &cfg); err == nil {
        w.BookmarkLinks = cfg.Links
    }
    if w.BookmarkLinks == nil {
        w.BookmarkLinks = []BookmarkLink{}
    }
}
```

4. Neue Funktion `GetWidgetByID` nach `GetAllWidgets`:
```go
func GetWidgetByID(id int) (Widget, error) {
    var w Widget
    row := DB.QueryRow(`SELECT id, type, name, config, profile, sort_order FROM widgets WHERE id = ?`, id)
    if err := row.Scan(&w.ID, &w.Type, &w.Name, &w.Config, &w.Profile, &w.SortOrder); err != nil {
        return w, err
    }
    populateWidgetFields(&w)
    return w, nil
}
```

5. Neue Bookmark-Mutations-Funktionen am Ende von db.go:
```go
func AddBookmarkLink(widgetID int, link BookmarkLink) error {
    var configStr string
    if err := DB.QueryRow(`SELECT config FROM widgets WHERE id = ?`, widgetID).Scan(&configStr); err != nil {
        return err
    }
    var cfg struct {
        Layout string         `json:"layout"`
        Links  []BookmarkLink `json:"links"`
    }
    _ = json.Unmarshal([]byte(configStr), &cfg)
    if cfg.Layout == "" { cfg.Layout = "grid" }
    cfg.Links = append(cfg.Links, link)
    newConfig, err := json.Marshal(cfg)
    if err != nil { return err }
    s := string(newConfig)
    return UpdateWidget(widgetID, nil, &s, nil)
}

func DeleteBookmarkLink(widgetID, idx int) error {
    var configStr string
    if err := DB.QueryRow(`SELECT config FROM widgets WHERE id = ?`, widgetID).Scan(&configStr); err != nil {
        return err
    }
    var cfg struct {
        Layout string         `json:"layout"`
        Links  []BookmarkLink `json:"links"`
    }
    _ = json.Unmarshal([]byte(configStr), &cfg)
    if idx < 0 || idx >= len(cfg.Links) {
        return fmt.Errorf("bookmark index out of range")
    }
    cfg.Links = append(cfg.Links[:idx], cfg.Links[idx+1:]...)
    newConfig, err := json.Marshal(cfg)
    if err != nil { return err }
    s := string(newConfig)
    return UpdateWidget(widgetID, nil, &s, nil)
}
```

6. Notes-Tabelle in `InitDB()` nach der `todos`-Tabelle:
```go
`CREATE TABLE IF NOT EXISTS notes (
    widget_id  INTEGER PRIMARY KEY REFERENCES widgets(id) ON DELETE CASCADE,
    content    TEXT NOT NULL DEFAULT '',
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);`,
```

7. Notes DB-Funktionen am Ende:
```go
func GetNote(widgetID int) (string, error) {
    var content string
    err := DB.QueryRow(`SELECT content FROM notes WHERE widget_id = ?`, widgetID).Scan(&content)
    if err == sql.ErrNoRows { return "", nil }
    return content, err
}

func SaveNote(widgetID int, content string) error {
    _, err := DB.Exec(`
        INSERT INTO notes (widget_id, content, updated_at) VALUES (?, ?, datetime('now'))
        ON CONFLICT(widget_id) DO UPDATE SET content=excluded.content, updated_at=excluded.updated_at`,
        widgetID, content)
    return err
}
```

8. Analytics-Struct und -Funktion:
```go
type ClickStat struct {
    ServiceID   int
    ServiceName string
    ServiceURL  string
    ServiceIcon string
    ClickCount  int
    LastClicked string
    Profile     string
}

func GetTopClicks(profile string, limit int) ([]ClickStat, error) {
    var rows *sql.Rows
    var err error
    if profile != "" {
        rows, err = DB.Query(`
            SELECT sc.service_id, s.name, s.url, COALESCE(s.icon,''),
                   sc.click_count, COALESCE(sc.last_clicked,''), sc.profile
            FROM service_clicks sc JOIN services s ON s.id = sc.service_id
            WHERE sc.profile = ?
            ORDER BY sc.click_count DESC LIMIT ?`, profile, limit)
    } else {
        rows, err = DB.Query(`
            SELECT sc.service_id, s.name, s.url, COALESCE(s.icon,''),
                   sc.click_count, COALESCE(sc.last_clicked,''), sc.profile
            FROM service_clicks sc JOIN services s ON s.id = sc.service_id
            ORDER BY sc.click_count DESC LIMIT ?`, limit)
    }
    if err != nil { return nil, err }
    defer rows.Close()
    var stats []ClickStat
    for rows.Next() {
        var s ClickStat
        if err := rows.Scan(&s.ServiceID, &s.ServiceName, &s.ServiceURL, &s.ServiceIcon,
            &s.ClickCount, &s.LastClicked, &s.Profile); err != nil {
            return nil, err
        }
        stats = append(stats, s)
    }
    return stats, nil
}
```

---

## Neue Datei: internal/api/bookmarks.go

```go
package api

import (
    "log"
    "net/http"
    "strconv"

    "github.com/go-chi/chi/v5"
    "git.zk35.de/secalpha/homeport/internal/db"
)

func renderBookmarksWidget(w http.ResponseWriter, widgetID int) {
    widget, err := db.GetWidgetByID(widgetID)
    if err != nil {
        http.Error(w, "Not Found", http.StatusNotFound)
        return
    }
    if err := IndexTmpl.ExecuteTemplate(w, "widget_bookmarks", widget); err != nil {
        log.Printf("renderBookmarksWidget error: %v", err)
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
    }
}

// POST /api/widgets/{id}/bookmark  (form: name, url)
func HandleAddBookmark(w http.ResponseWriter, r *http.Request) {
    id, err := strconv.Atoi(chi.URLParam(r, "id"))
    if err != nil { http.Error(w, "Bad Request", http.StatusBadRequest); return }
    if err := r.ParseForm(); err != nil { http.Error(w, "Bad Request", http.StatusBadRequest); return }
    link := db.BookmarkLink{Name: r.FormValue("name"), URL: r.FormValue("url")}
    if link.URL == "" { http.Error(w, "Bad Request", http.StatusBadRequest); return }
    if link.Name == "" { link.Name = link.URL }
    if err := db.AddBookmarkLink(id, link); err != nil {
        log.Printf("AddBookmarkLink error: %v", err)
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
        return
    }
    renderBookmarksWidget(w, id)
}

// DELETE /api/widgets/{id}/bookmark/{idx}
func HandleDeleteBookmark(w http.ResponseWriter, r *http.Request) {
    id, err := strconv.Atoi(chi.URLParam(r, "id"))
    if err != nil { http.Error(w, "Bad Request", http.StatusBadRequest); return }
    idx, err := strconv.Atoi(chi.URLParam(r, "idx"))
    if err != nil { http.Error(w, "Bad Request", http.StatusBadRequest); return }
    if err := db.DeleteBookmarkLink(id, idx); err != nil {
        log.Printf("DeleteBookmarkLink error: %v", err)
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
        return
    }
    renderBookmarksWidget(w, id)
}
```

---

## Neue Datei: internal/api/notes.go

```go
package api

import (
    "encoding/json"
    "log"
    "net/http"
    "strconv"

    "github.com/go-chi/chi/v5"
    "git.zk35.de/secalpha/homeport/internal/db"
)

// PUT /api/notes/{id}  body: {"content":"..."}
func HandleSaveNote(w http.ResponseWriter, r *http.Request) {
    id, err := strconv.Atoi(chi.URLParam(r, "id"))
    if err != nil { http.Error(w, "Bad Request", http.StatusBadRequest); return }
    var body struct {
        Content string `json:"content"`
    }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        http.Error(w, "Bad Request", http.StatusBadRequest)
        return
    }
    if err := db.SaveNote(id, body.Content); err != nil {
        log.Printf("SaveNote error: %v", err)
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusNoContent)
}
```

---

## Neue Datei: internal/api/analytics.go

```go
package api

import (
    "html/template"
    "log"
    "net/http"

    "git.zk35.de/secalpha/homeport/internal/db"
)

var AnalyticsTmpl *template.Template

type AnalyticsData struct {
    Stats    []db.ClickStat
    Profile  string
    Profiles []db.Profile
    Prefs    *db.UserPreferences
}

// GET /manage/analytics
func HandleAnalytics(w http.ResponseWriter, r *http.Request) {
    filterProfile := r.URL.Query().Get("profile")
    stats, err := db.GetTopClicks(filterProfile, 25)
    if err != nil {
        log.Printf("GetTopClicks error: %v", err)
        stats = nil
    }
    profiles, _ := db.GetProfiles()
    defaultProf, _ := db.GetDefaultProfile()
    var prefs *db.UserPreferences
    if defaultProf != nil {
        prefs, _ = db.GetUserPreferences(defaultProf.Slug)
    }
    if prefs == nil {
        prefs = &db.UserPreferences{Theme: "dark", AccentColor: "#6366f1"}
    }
    data := AnalyticsData{Stats: stats, Profile: filterProfile, Profiles: profiles, Prefs: prefs}
    if err := AnalyticsTmpl.ExecuteTemplate(w, "base.html", data); err != nil {
        log.Printf("Analytics template error: %v", err)
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
    }
}
```

---

## internal/api/index.go – Änderungen

1. In `tmplFuncs` die `inc`-Funktion ergänzen:
```go
var tmplFuncs = template.FuncMap{
    "isImgURL": isImgURL,
    "hexToRGB": hexToRGB,
    "inc": func(i int) int { return i + 1 },
}
```

2. In `InitTemplates()` nach der ManageTmpl-Initialisierung:
```go
AnalyticsTmpl, err = template.New("").Funcs(tmplFuncs).ParseFS(fs,
    "templates/base.html",
    "templates/analytics.html",
    "templates/partials/*.html",
)
if err != nil {
    log.Fatalf("Error parsing analytics templates: %v", err)
}
```

3. Im `for i := range widgets` switch-Block nach dem `todo`-Case:
```go
case "notes":
    if content, err := db.GetNote(widgets[i].ID); err == nil {
        widgets[i].NoteContent = content
    }
```

---

## internal/api/manage.go – HandleAddWidget

Im `switch widgetType` nach dem `todo`-Case ergänzen:
```go
case "bookmarks":
    err = db.AddWidgetTyped(name, "bookmarks", `{"layout":"grid","links":[]}`, profile)
case "notes":
    err = db.AddWidgetTyped(name, "notes", `{}`, profile)
```

---

## cmd/homeport/main.go – neue Routen

Nach `r.Delete("/api/todos/{id}", api.HandleDeleteTodo)`:
```go
r.Post("/api/widgets/{id}/bookmark", api.HandleAddBookmark)
r.Delete("/api/widgets/{id}/bookmark/{idx}", api.HandleDeleteBookmark)
r.Put("/api/notes/{id}", api.HandleSaveNote)
```

Im `/manage`-Block nach `r.Get("/", api.HandleManage)`:
```go
r.Get("/analytics", api.HandleAnalytics)
```

---

## Neue Datei: assets/templates/partials/widget_bookmarks.html

```html
{{define "widget_bookmarks"}}
<div class="widget widget-bookmarks" id="widget-bookmarks-{{.ID}}">
  <h3 class="widget-title">{{.Name}}</h3>
  <div class="bookmarks-list">
    {{range $idx, $link := .BookmarkLinks}}
    <div class="bookmark-item">
      <a href="{{$link.URL}}" target="_blank" rel="noopener" class="bookmark-link">
        {{if $link.Icon}}
          <img src="{{$link.Icon}}" width="16" height="16" alt="" class="bookmark-favicon" onerror="this.style.display='none'">
        {{else}}
          <img src="/api/favicon?url={{$link.URL}}" width="16" height="16" alt="" class="bookmark-favicon" onerror="this.style.display='none'">
        {{end}}
        <span class="bookmark-name">{{$link.Name}}</span>
      </a>
      <button class="btn-icon bookmark-del" title="Löschen"
        hx-delete="/api/widgets/{{$.ID}}/bookmark/{{$idx}}"
        hx-target="#widget-bookmarks-{{$.ID}}"
        hx-swap="outerHTML">🗑️</button>
    </div>
    {{else}}
    <p class="bookmark-empty">Noch keine Lesezeichen.</p>
    {{end}}
  </div>
  <form class="bookmark-add-form"
    hx-post="/api/widgets/{{.ID}}/bookmark"
    hx-target="#widget-bookmarks-{{.ID}}"
    hx-swap="outerHTML"
    hx-on::after-request="this.reset()">
    <input type="text" name="name" placeholder="Name" class="bookmark-input-name" autocomplete="off">
    <input type="url" name="url" placeholder="https://…" class="bookmark-input-url" autocomplete="off" required>
    <button type="submit" class="btn-sm">+</button>
  </form>
</div>
{{end}}
```

---

## Neue Datei: assets/templates/partials/widget_notes.html

```html
{{define "widget_notes"}}
<div class="widget widget-notes" id="widget-notes-{{.ID}}">
  <div class="widget-title-row">
    <h3 class="widget-title">{{.Name}}</h3>
    <span id="notes-status-{{.ID}}" class="notes-status"></span>
  </div>
  <textarea id="notes-ta-{{.ID}}" class="notes-textarea" placeholder="Notizen…">{{.NoteContent}}</textarea>
  <script>
  (function(){
    var ta = document.getElementById('notes-ta-{{.ID}}');
    var st = document.getElementById('notes-status-{{.ID}}');
    var timer;
    ta.addEventListener('input', function() {
      st.textContent = '…';
      clearTimeout(timer);
      timer = setTimeout(function() {
        fetch('/api/notes/{{.ID}}', {
          method: 'PUT',
          headers: {'Content-Type': 'application/json'},
          body: JSON.stringify({content: ta.value})
        }).then(function(r) {
          st.textContent = r.ok ? '✓' : '✗';
          setTimeout(function(){ st.textContent = ''; }, 2000);
        }).catch(function(){ st.textContent = '✗'; });
      }, 500);
    });
  })();
  </script>
</div>
{{end}}
```

---

## Neue Datei: assets/templates/analytics.html

```html
{{define "content"}}
<style>
.analytics-wrap { max-width: 900px; margin: 0 auto; padding: 1.5rem 1rem; }
.analytics-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 1.5rem; gap: 1rem; flex-wrap: wrap; }
.analytics-header h2 { margin: 0; }
.analytics-filter { display: flex; align-items: center; gap: 0.5rem; }
.analytics-filter select { background: var(--glass); border: 1px solid var(--border); border-radius: 6px; padding: 0.3rem 0.5rem; color: var(--fg); font-size: 0.9rem; }
.analytics-table { width: 100%; border-collapse: collapse; }
.analytics-table th, .analytics-table td { padding: 0.55rem 0.75rem; text-align: left; border-bottom: 1px solid var(--border); }
.analytics-table th { color: var(--sub); font-size: 0.78rem; font-weight: 600; text-transform: uppercase; }
.analytics-table tr:hover td { background: var(--glass); }
.a-rank { color: var(--sub); font-size: 0.85rem; width: 2rem; }
.a-count { font-weight: 700; color: var(--accent); }
.a-profile-tag { font-size: 0.75rem; color: var(--sub); background: var(--glass); padding: 2px 6px; border-radius: 4px; }
.a-date { color: var(--sub); font-size: 0.8rem; white-space: nowrap; }
.a-svc { display: flex; align-items: center; gap: 0.5rem; }
.a-favicon { width: 16px; height: 16px; border-radius: 3px; flex-shrink: 0; }
.analytics-empty { color: var(--sub); text-align: center; padding: 3rem; }
</style>

<div class="analytics-wrap">
  <div class="analytics-header">
    <h2>📊 Analytics</h2>
    <div class="analytics-filter">
      <label style="color:var(--sub);font-size:0.9rem">Profil:</label>
      <select onchange="location.href='/manage/analytics?profile='+this.value">
        <option value="" {{if eq .Profile ""}}selected{{end}}>Alle</option>
        {{range .Profiles}}
        <option value="{{.Slug}}" {{if eq $.Profile .Slug}}selected{{end}}>{{.Name}}</option>
        {{end}}
      </select>
      <a href="/manage" style="color:var(--sub);font-size:0.85rem;margin-left:1rem">← Manage</a>
    </div>
  </div>

  {{if .Stats}}
  <table class="analytics-table">
    <thead>
      <tr>
        <th class="a-rank">#</th>
        <th>Dienst</th>
        <th>Profil</th>
        <th>Klicks</th>
        <th>Zuletzt</th>
      </tr>
    </thead>
    <tbody>
    {{range $i, $s := .Stats}}
    <tr>
      <td class="a-rank">{{inc $i}}</td>
      <td>
        <div class="a-svc">
          {{if isImgURL $s.ServiceIcon}}
            <img src="{{$s.ServiceIcon}}" class="a-favicon" alt="" onerror="this.style.display='none'">
          {{else if $s.ServiceIcon}}
            <span>{{$s.ServiceIcon}}</span>
          {{else}}
            <img src="/api/favicon?url={{$s.ServiceURL}}" class="a-favicon" alt="" onerror="this.style.display='none'">
          {{end}}
          <a href="{{$s.ServiceURL}}" target="_blank" rel="noopener">{{$s.ServiceName}}</a>
        </div>
      </td>
      <td><span class="a-profile-tag">{{$s.Profile}}</span></td>
      <td class="a-count">{{$s.ClickCount}}</td>
      <td class="a-date">{{$s.LastClicked}}</td>
    </tr>
    {{end}}
    </tbody>
  </table>
  {{else}}
  <p class="analytics-empty">Noch keine Klicks erfasst.<br>
  Dienste müssen über <code>/r/{id}?p={profile}</code> geöffnet werden – das passiert automatisch wenn sie auf dem Dashboard angeklickt werden.</p>
  {{end}}
</div>
{{end}}
```

---

## assets/templates/index.html – Widget-Switch erweitern

Im Widget-Switch-Block (Zeile mit `{{if eq .Type "weather"}}...`), neue Zeilen nach `{{else if eq .Type "todo"}}{{template "widget_todo" .}}` ergänzen:
```
{{else if eq .Type "bookmarks"}}{{template "widget_bookmarks" .}}
{{else if eq .Type "notes"}}{{template "widget_notes" .}}
```

---

## assets/templates/manage.html – Änderungen

1. Im `<select name="widget_type" id="widget-type-select">` ergänzen:
```html
<option value="bookmarks">Lesezeichen</option>
<option value="notes">Notizen</option>
```

2. Nach `<div id="widget-todo-fields"...>` zwei neue Divs ergänzen:
```html
<div id="widget-bookmarks-fields" style="display:none">
    <p style="font-size:0.8rem;color:var(--sub);margin:0">Links werden direkt im Widget verwaltet.</p>
</div>
<div id="widget-notes-fields" style="display:none">
    <p style="font-size:0.8rem;color:var(--sub);margin:0">Inhalt wird direkt im Widget bearbeitet.</p>
</div>
```

3. In der JS-Funktion `updateWidgetForm()`: Die Funktion versteckt je nach Widget-Typ die nicht benötigten Felder. Suche die bestehende Logik und ergänze `bookmarks` und `notes` so dass deren Felder gezeigt / alle anderen versteckt werden. Das Muster ist dasselbe wie bei `todo` (zeige nur den zugehörigen div).

4. Analytics-Link: Suche in manage.html den Abschnitt mit `<h1>` oder `<h2>` Überschrift der Seite und ergänze einen Link:
```html
<a href="/manage/analytics" style="font-size:0.85rem;color:var(--sub)">📊 Analytics</a>
```
Stelle es gut sichtbar in den Header-Bereich.

---

## assets/static/style.css – neue CSS-Klassen am Ende

```css
/* ─── Bookmarks Widget ─────────────────────────────── */
.bookmarks-list { display: flex; flex-direction: column; gap: 0.35rem; }
.bookmark-item { display: flex; align-items: center; gap: 0.4rem; }
.bookmark-link { display: flex; align-items: center; gap: 0.5rem; flex: 1; text-decoration: none; color: var(--fg); padding: 0.25rem 0.4rem; border-radius: 6px; transition: background 0.15s; overflow: hidden; }
.bookmark-link:hover { background: var(--glass); }
.bookmark-favicon { border-radius: 3px; flex-shrink: 0; }
.bookmark-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 0.9rem; }
.bookmark-empty { color: var(--sub); font-size: 0.85rem; margin: 0.25rem 0; }
.bookmark-add-form { display: flex; gap: 0.4rem; margin-top: 0.6rem; flex-wrap: wrap; }
.bookmark-input-name { flex: 1; min-width: 80px; }
.bookmark-input-url { flex: 2; min-width: 120px; }

/* ─── Notes Widget ──────────────────────────────────── */
.widget-title-row { display: flex; align-items: baseline; gap: 0.5rem; margin-bottom: 0.5rem; }
.widget-title-row .widget-title { margin-bottom: 0; }
.notes-textarea { width: 100%; min-height: 120px; resize: vertical; background: rgba(255,255,255,0.05); border: 1px solid var(--border); border-radius: 8px; padding: 0.5rem 0.6rem; color: var(--fg); font-family: inherit; font-size: 0.9rem; box-sizing: border-box; line-height: 1.5; }
.notes-textarea:focus { outline: none; border-color: var(--accent); }
.notes-status { font-size: 0.75rem; color: var(--sub); flex-shrink: 0; }
```

---

## Build und Test

```bash
cd /home/debian/new_pro/homeport
go build ./...
go test ./...
```

---

## Git Commit

```bash
git add -A
git commit -m "feat: Bookmarks Widget (#26), Notes Widget (#30), Analytics (#36)"
```
