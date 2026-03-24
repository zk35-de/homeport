package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

func InitDB(dbPath string) error {
	if dbPath == "" {
		dbPath = "./data/homeport.db"
	}
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create db directory: %w", err)
	}

	// Encode foreign_keys pragma directly in DSN so it applies to every connection.
	dsn := dbPath + "?_pragma=foreign_keys(on)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	if dbPath == ":memory:" {
		dsn = ":memory:?_pragma=foreign_keys(on)"
	}

	var err error
	DB, err = sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("failed to open db: %w", err)
	}

	if err := DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping db: %w", err)
	}

	if err := Migrate(DB); err != nil {
		return fmt.Errorf("failed to migrate db: %w", err)
	}

	// Seed default profiles if table is empty
	var count int
	DB.QueryRow(`SELECT COUNT(*) FROM profiles`).Scan(&count)
	if count == 0 {
		DB.Exec(`INSERT INTO profiles (slug, name, is_default, sort_order) VALUES ('markus', 'Markus', 1, 0)`)
		DB.Exec(`INSERT INTO profiles (slug, name, is_default, sort_order) VALUES ('andrea', 'Andrea', 0, 1)`)
	}

	return nil
}

func ReinitDB(dbPath string) error {
	if DB != nil {
		DB.Close()
	}
	return InitDB(dbPath)
}
