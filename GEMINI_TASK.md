# Task: Backup & Restore für homeport (#48)

## Projekt-Kontext
Go + HTMX + SQLite Startpage. Alle Assets sind via go:embed eingebettet.
- DB-Pfad: aus `cfg.DBPath` (internal/config)
- SQLite-Treiber: `modernc.org/sqlite` (pure Go, kein CGO)
- Router: chi v5
- Templates: `assets/templates/manage.html` + Partials

## Was zu implementieren ist

### 1. `internal/backup/backup.go` (neues Package)

```go
package backup

// VacuumInto erstellt einen konsistenten SQLite-Snapshot via VACUUM INTO.
// Gibt den Pfad der erstellten Datei zurück.
func CreateSnapshot(dbPath, destDir string) (string, error)

// Validate prüft ob eine Datei eine gültige homeport-DB ist (SQLite + Pflicht-Tabellen).
func Validate(path string) error

// Rotate löscht älteste Backup-Dateien wenn count > maxKeep.
func Rotate(dir string, maxKeep int) error

// ScheduledBackup startet eine Goroutine die alle `interval` ein Backup macht.
// interval=0 → kein automatisches Backup.
// Gibt einen Stop-Channel zurück.
func ScheduledBackup(dbPath, destDir string, interval time.Duration, maxKeep int) chan struct{}
```

**Wichtig für VACUUM INTO:**
```go
// modernc sqlite unterstützt VACUUM INTO direkt:
db.Exec("VACUUM INTO ?", destPath)
// db muss via database/sql geöffnet sein (nicht der global DB aus internal/db)
// Öffne eine zweite Read-Only Connection zur DB für den Snapshot
```

**Dateinamen-Format:** `homeport_backup_20260321_143022.db`

**Validate** prüft: Datei öffnbar als SQLite + folgende Tabellen existieren:
`categories`, `services`, `widgets`, `user_preferences`

### 2. Config erweitern (`internal/config/config.go`)

Neue Felder:
```go
BackupDir      string  // HOMEPORT_BACKUP_DIR, default: "./data/backups"
BackupInterval string  // HOMEPORT_BACKUP_INTERVAL, default: "" (aus)
BackupMaxKeep  int     // HOMEPORT_BACKUP_MAX_KEEP, default: 7
```

Parsing: `BackupInterval` via `time.ParseDuration`, leerer String = deaktiviert.

### 3. Routen in `cmd/homeport/main.go`

```go
// Im /manage Block (kein Auth-Guard nötig, manage ist bereits intern):
r.Get("/backup", api.HandleBackupDownload)
r.Post("/restore", api.HandleRestore)
```

ScheduledBackup-Goroutine im main() starten (nach db.Init):
```go
if cfg.BackupInterval != "" {
    d, _ := time.ParseDuration(cfg.BackupInterval)
    backup.ScheduledBackup(cfg.DBPath, cfg.BackupDir, d, cfg.BackupMaxKeep)
}
```

### 4. Handler in `internal/api/backup_api.go` (neue Datei)

**HandleBackupDownload** (`GET /manage/backup`):
- Ruft `backup.CreateSnapshot(cfg.DBPath, cfg.BackupDir)` auf
- Setzt Header: `Content-Disposition: attachment; filename="homeport_backup_DATUM.db"`
- Streamt die Datei, löscht sie danach NICHT (bleibt als lokales Backup)

**HandleRestore** (`POST /manage/restore`):
- Multipart-Form, Feld `file`
- Max 100MB (`r.ParseMultipartForm(100 << 20)`)
- Schreibt in `/tmp/homeport_restore_*.db` (temp)
- Ruft `backup.Validate(tempPath)` auf → bei Fehler: HTTP 400 + Fehlermeldung
- Atomic swap: `os.Rename(cfg.DBPath, cfg.DBPath+".bak")`, dann `os.Rename(tempPath, cfg.DBPath)`
- `db.Reinit(cfg.DBPath)` (falls vorhanden) oder Server-Neustart-Hinweis
- Response: redirect zu `/manage#backup` mit Flash-Message ODER JSON `{"ok":true}`

**Wichtig:** cfg muss in den Handlern verfügbar sein.
Schau wie es in `api/` gemacht wird – vermutlich via Package-Variable oder Init-Funktion.
Schau in `internal/api/` wie andere Handler auf Config zugreifen.

### 5. `assets/templates/manage.html` – Backup-Sektion

Füge nach der Shortener-Sektion ein:

```html
<section class="manage-section" id="backup">
  <h2>Backup & Restore</h2>

  <!-- Manueller Backup-Download -->
  <div style="display:flex;gap:1rem;align-items:center;flex-wrap:wrap;margin-bottom:1rem">
    <a href="/manage/backup" class="btn-sm" download>💾 Backup herunterladen</a>
    <span style="font-size:0.8rem;color:var(--sub)">Lädt homeport.db als Snapshot</span>
  </div>

  <!-- Restore-Upload -->
  <form method="post" action="/manage/restore" enctype="multipart/form-data"
        onsubmit="return confirm('⚠️ Restore überschreibt ALLE Daten. Fortfahren?')">
    <div style="display:flex;gap:0.5rem;align-items:flex-end;flex-wrap:wrap">
      <div class="form-group" style="margin-bottom:0">
        <label>DB-Datei wiederherstellen</label>
        <input type="file" name="file" accept=".db" required>
      </div>
      <button type="submit" class="btn-sm" style="background:var(--fehler)">⚠️ Restore</button>
    </div>
  </form>
</section>
```

## Bestehende Code-Struktur (wichtig!)

- `internal/db/db.go` exportiert `var DB *sql.DB` und `func Init(path string) error`
- Config wird in main() geladen: `cfg := config.Load()`
- Handler bekommen cfg via `api.SetConfig(cfg)` oder ähnlich – prüfe wie es gemacht wird
- Schaue in `internal/api/favicon.go` oder `internal/api/health.go` wie Config dort genutzt wird

## Was NICHT zu tun ist
- Kein CGO
- Keine neuen Dependencies (nur stdlib + was schon in go.mod ist)
- Keine Tests schreiben (die kommen separat)
- manage.html nicht komplett umschreiben, nur die Backup-Sektion hinzufügen

## Go.mod prüfen
`cat go.mod` um zu sehen was verfügbar ist.
