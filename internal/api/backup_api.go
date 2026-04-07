package api

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/zk35-de/homeport/internal/backup"
	"github.com/zk35-de/homeport/internal/db"
)

// HandleBackupDownload triggers a manual backup and streams it.
// GET /manage/backup
func (s *Server) HandleBackupDownload(w http.ResponseWriter, r *http.Request) {
	if s.Config == nil {
		http.Error(w, "Server not configured", http.StatusInternalServerError)
		return
	}
	path, err := backup.CreateSnapshot(s.Config.DBPath, s.Config.BackupDir)
	if err != nil {
		slog.Error("creating backup", "err", err)
		http.Error(w, "Failed to create backup", http.StatusInternalServerError)
		return
	}

	// Also perform rotation
	_ = backup.Rotate(s.Config.BackupDir, s.Config.BackupMaxKeep)

	f, err := os.Open(path)
	if err != nil {
		slog.Error("opening backup file", "err", err)
		http.Error(w, "Failed to open backup", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	info, _ := f.Stat()
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(path)))
	w.Header().Set("Content-Type", "application/x-sqlite3")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	http.ServeContent(w, r, filepath.Base(path), info.ModTime(), f)
}

// HandleRestore restores a database from an uploaded file.
// POST /manage/restore
func (s *Server) HandleRestore(w http.ResponseWriter, r *http.Request) {
	if s.Config == nil {
		http.Error(w, "Server not configured", http.StatusInternalServerError)
		return
	}
	// Max 100MB
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Write temp file into the same directory as the target DB so os.Rename
	// stays on the same filesystem (cross-device rename fails in containers).
	tempFile, err := os.CreateTemp(filepath.Dir(s.Config.DBPath), "homeport_restore_*.db")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	tempPath := tempFile.Name()
	defer tempFile.Close()
	// Cleanup temp file only if it still exists (may be renamed away on success)
	defer func() { os.Remove(tempPath) }()

	if _, err := io.Copy(tempFile, file); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	tempFile.Close()

	if err := backup.Validate(tempPath); err != nil {
		slog.Error("restore validation failed", "err", err)
		http.Error(w, "Invalid database file: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Atomic swap: move old DB to .bak, move new DB to DBPath
	bakPath := s.Config.DBPath + ".bak"
	if err := os.Rename(s.Config.DBPath, bakPath); err != nil {
		slog.Error("backup old DB during restore", "err", err)
		http.Error(w, "Failed to swap database", http.StatusInternalServerError)
		return
	}

	if err := os.Rename(tempPath, s.Config.DBPath); err != nil {
		slog.Error("move restored DB", "err", err)
		// Try to restore the backup
		_ = os.Rename(bakPath, s.Config.DBPath)
		http.Error(w, "Failed to restore database", http.StatusInternalServerError)
		return
	}

	// Reinit DB
	if err := db.ReinitDB(s.Config.DBPath); err != nil {
		slog.Error("reinit DB after restore", "err", err)
		http.Error(w, "Database restored but failed to re-initialize. Please restart the server.", http.StatusInternalServerError)
		return
	}

	// Success redirect
	http.Redirect(w, r, "/manage#backup", http.StatusSeeOther)
}
