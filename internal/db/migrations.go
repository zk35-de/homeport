package db

import (
	"database/sql"
	"fmt"
)

// migrations holds all schema changes in order.
// Each entry is applied only if its version > current PRAGMA user_version.
// ALTER TABLE steps use ignoreError to handle columns that already exist (idempotent upgrades).
var migrations = []struct {
	version     int
	sql         string
	ignoreError bool // true for ALTER TABLE – column may already exist on legacy DBs
}{
	// v1 – initial schema (all CREATE TABLE IF NOT EXISTS)
	{1, `CREATE TABLE IF NOT EXISTS categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		layout TEXT NOT NULL DEFAULT 'tiles',
		color TEXT NOT NULL DEFAULT 'indigo',
		sort_order INTEGER NOT NULL DEFAULT 0
	)`, false},
	{1, `CREATE TABLE IF NOT EXISTS services (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		url TEXT NOT NULL,
		icon TEXT DEFAULT '',
		description TEXT DEFAULT '',
		status_check TEXT DEFAULT '',
		sort_order INTEGER NOT NULL DEFAULT 0
	)`, false},
	{1, `CREATE TABLE IF NOT EXISTS visibility (
		service_id INTEGER NOT NULL REFERENCES services(id) ON DELETE CASCADE,
		profile TEXT NOT NULL,
		PRIMARY KEY (service_id, profile)
	)`, false},
	{1, `CREATE TABLE IF NOT EXISTS service_status (
		service_id INTEGER PRIMARY KEY REFERENCES services(id) ON DELETE CASCADE,
		alive INTEGER,
		last_check DATETIME
	)`, false},
	{1, `CREATE TABLE IF NOT EXISTS widgets (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT NOT NULL DEFAULT 'ical',
		name TEXT NOT NULL,
		config TEXT NOT NULL,
		profile TEXT NOT NULL DEFAULT 'all',
		sort_order INTEGER NOT NULL DEFAULT 0
	)`, false},
	{1, `CREATE TABLE IF NOT EXISTS widget_cache (
		widget_id INTEGER PRIMARY KEY REFERENCES widgets(id) ON DELETE CASCADE,
		data TEXT NOT NULL,
		fetched_at DATETIME NOT NULL
	)`, false},
	{1, `CREATE TABLE IF NOT EXISTS discovery_inbox (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		container_id TEXT NOT NULL UNIQUE,
		suggested    TEXT NOT NULL,
		seen_at      DATETIME NOT NULL DEFAULT (datetime('now')),
		ignored      INTEGER NOT NULL DEFAULT 0
	)`, false},
	{1, `CREATE TABLE IF NOT EXISTS user_settings (
		profile        TEXT PRIMARY KEY,
		search_engine  TEXT NOT NULL DEFAULT 'https://duckduckgo.com/'
	)`, false},
	{1, `CREATE TABLE IF NOT EXISTS user_preferences (
		profile       TEXT PRIMARY KEY,
		theme         TEXT NOT NULL DEFAULT 'dark',
		accent_color  TEXT NOT NULL DEFAULT '#6366f1',
		search_engine TEXT NOT NULL DEFAULT 'https://duckduckgo.com/',
		background    TEXT NOT NULL DEFAULT 'aurora',
		language      TEXT NOT NULL DEFAULT 'de',
		layout        TEXT NOT NULL DEFAULT 'grid'
	)`, false},
	{1, `CREATE TABLE IF NOT EXISTS profiles (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		slug       TEXT NOT NULL UNIQUE,
		name       TEXT NOT NULL,
		is_default INTEGER NOT NULL DEFAULT 0,
		sort_order INTEGER NOT NULL DEFAULT 0
	)`, false},
	{1, `CREATE TABLE IF NOT EXISTS todos (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		widget_id  INTEGER NOT NULL REFERENCES widgets(id) ON DELETE CASCADE,
		text       TEXT NOT NULL,
		done       INTEGER NOT NULL DEFAULT 0,
		due_date   TEXT NOT NULL DEFAULT '',
		sort_order INTEGER NOT NULL DEFAULT 0
	)`, false},
	{1, `CREATE TABLE IF NOT EXISTS service_clicks (
		service_id  INTEGER NOT NULL REFERENCES services(id) ON DELETE CASCADE,
		profile     TEXT NOT NULL,
		click_count INTEGER NOT NULL DEFAULT 1,
		last_clicked DATETIME NOT NULL DEFAULT (datetime('now')),
		PRIMARY KEY (service_id, profile)
	)`, false},
	{1, `CREATE TABLE IF NOT EXISTS notes (
		widget_id  INTEGER PRIMARY KEY REFERENCES widgets(id) ON DELETE CASCADE,
		content    TEXT NOT NULL DEFAULT '',
		updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`, false},
	{1, `CREATE TABLE IF NOT EXISTS pages (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		profile    TEXT NOT NULL,
		name       TEXT NOT NULL,
		icon       TEXT NOT NULL DEFAULT '📄',
		sort_order INTEGER NOT NULL DEFAULT 0
	)`, false},
	{1, `CREATE TABLE IF NOT EXISTS user_auth (
		profile    TEXT PRIMARY KEY REFERENCES profiles(slug) ON DELETE CASCADE,
		password   TEXT NOT NULL,
		is_admin   INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`, false},
	{1, `CREATE TABLE IF NOT EXISTS sessions (
		token      TEXT PRIMARY KEY,
		profile    TEXT NOT NULL REFERENCES profiles(slug) ON DELETE CASCADE,
		expires_at DATETIME NOT NULL,
		created_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`, false},
	{1, `CREATE TABLE IF NOT EXISTS discovery_sources (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		type       TEXT NOT NULL,
		name       TEXT NOT NULL,
		url        TEXT NOT NULL,
		token      TEXT NOT NULL DEFAULT '',
		enabled    INTEGER NOT NULL DEFAULT 1,
		interval   INTEGER NOT NULL DEFAULT 60,
		created_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`, false},

	// v2-v11 – column additions (ignoreError=true: column may already exist on legacy DBs)
	{2, `ALTER TABLE widgets ADD COLUMN visible INTEGER NOT NULL DEFAULT 1`, true},
	{3, `ALTER TABLE user_preferences ADD COLUMN custom_css TEXT NOT NULL DEFAULT ''`, true},
	{4, `ALTER TABLE categories ADD COLUMN col_span INTEGER NOT NULL DEFAULT 1`, true},
	{5, `ALTER TABLE categories ADD COLUMN sort_mode TEXT NOT NULL DEFAULT 'manual'`, true},
	{6, `ALTER TABLE user_preferences ADD COLUMN background_mode TEXT NOT NULL DEFAULT 'aurora'`, true},
	{7, `ALTER TABLE categories ADD COLUMN page_id INTEGER REFERENCES pages(id) ON DELETE SET NULL`, true},
	{8, `ALTER TABLE widgets ADD COLUMN page_id INTEGER REFERENCES pages(id) ON DELETE SET NULL`, true},
	{9, `ALTER TABLE categories ADD COLUMN public INTEGER NOT NULL DEFAULT 0`, true},
	{10, `ALTER TABLE discovery_inbox ADD COLUMN source_id INTEGER REFERENCES discovery_sources(id) ON DELETE SET NULL`, true},
	{11, `ALTER TABLE discovery_inbox ADD COLUMN external_id TEXT NOT NULL DEFAULT ''`, true},
}

// maxMigrationVersion is the highest version number in the migrations list.
const maxMigrationVersion = 11

// Migrate applies all pending migrations to db using PRAGMA user_version as the version tracker.
// It is safe to call on both fresh and existing (pre-migration) databases.
func Migrate(db *sql.DB) error {
	current, err := getUserVersion(db)
	if err != nil {
		return fmt.Errorf("migrate: get user_version: %w", err)
	}

	applied := make(map[int]bool)
	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		if _, err := db.Exec(m.sql); err != nil {
			if m.ignoreError {
				continue
			}
			return fmt.Errorf("migrate v%d: %w", m.version, err)
		}
		applied[m.version] = true
	}

	if len(applied) > 0 {
		if err := setUserVersion(db, maxMigrationVersion); err != nil {
			return fmt.Errorf("migrate: set user_version: %w", err)
		}
	}
	return nil
}

func getUserVersion(db *sql.DB) (int, error) {
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		return 0, err
	}
	return v, nil
}

func setUserVersion(db *sql.DB, v int) error {
	_, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, v))
	return err
}
