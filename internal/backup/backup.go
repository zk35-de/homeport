package backup

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	_ "modernc.org/sqlite"
)

// CreateSnapshot erstellt einen konsistenten SQLite-Snapshot via VACUUM INTO.
// Gibt den Pfad der erstellten Datei zurück.
func CreateSnapshot(dbPath, destDir string) (string, error) {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	destFileName := fmt.Sprintf("homeport_backup_%s.db", timestamp)
	destPath := filepath.Join(destDir, destFileName)

	// Öffne eine zweite Connection zur DB für den Snapshot.
	// modernc sqlite unterstützt VACUUM INTO direkt.
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return "", fmt.Errorf("failed to open source db: %w", err)
	}
	defer db.Close()

	if _, err := db.Exec("VACUUM INTO ?", destPath); err != nil {
		return "", fmt.Errorf("VACUUM INTO failed: %w", err)
	}

	return destPath, nil
}

// Validate prüft ob eine Datei eine gültige homeport-DB ist (SQLite + Pflicht-Tabellen).
func Validate(path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	tables := []string{"categories", "services", "widgets", "user_preferences"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("missing mandatory table: %s", table)
			}
			return fmt.Errorf("failed to query table %s: %w", table, err)
		}
	}

	return nil
}

// Rotate löscht älteste Backup-Dateien wenn count > maxKeep.
func Rotate(dir string, maxKeep int) error {
	if maxKeep <= 0 {
		return nil
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read backup directory: %w", err)
	}

	var backups []os.FileInfo
	for _, f := range files {
		if !f.IsDir() && filepath.Ext(f.Name()) == ".db" {
			info, err := f.Info()
			if err == nil {
				backups = append(backups, info)
			}
		}
	}

	if len(backups) <= maxKeep {
		return nil
	}

	// Sort by modification time, oldest first
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].ModTime().Before(backups[j].ModTime())
	})

	toDelete := len(backups) - maxKeep
	for i := 0; i < toDelete; i++ {
		path := filepath.Join(dir, backups[i].Name())
		if err := os.Remove(path); err != nil {
			slog.Error("failed to delete old backup", "path", path, "err", err)
		} else {
			slog.Info("deleted old backup", "path", path)
		}
	}

	return nil
}

// ScheduledBackup startet eine Goroutine die alle `interval` ein Backup macht.
// interval=0 → kein automatisches Backup.
// Gibt einen Stop-Channel zurück.
func ScheduledBackup(dbPath, destDir string, interval time.Duration, maxKeep int) chan struct{} {
	stop := make(chan struct{})
	if interval <= 0 {
		return stop
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				slog.Info("starting scheduled backup")
				path, err := CreateSnapshot(dbPath, destDir)
				if err != nil {
					slog.Error("scheduled backup failed", "err", err)
					continue
				}
				slog.Info("scheduled backup created", "path", path)

				if err := Rotate(destDir, maxKeep); err != nil {
					slog.Error("backup rotation failed", "err", err)
				}
			case <-stop:
				return
			}
		}
	}()

	return stop
}
