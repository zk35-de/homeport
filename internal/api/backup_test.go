package api_test

import (
	"bytes"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zk35-de/homeport/internal/api"
	"github.com/zk35-de/homeport/internal/backup"
	"github.com/zk35-de/homeport/internal/config"
	"github.com/zk35-de/homeport/internal/db"
)

// setupBackupTest creates a test environment with a real file-based DB whose
// path is known and stored in srv.Config, enabling backup/restore handler tests.
//
// Unlike setupTest (which uses a temp file but doesn't record the path in Config),
// this helper creates a temp directory so that the DB and any restore temp files
// live on the same filesystem — preventing the cross-device rename bug (#162).
func setupBackupTest(t *testing.T) (*api.Server, string, func()) {
	t.Helper()
	log.SetOutput(io.Discard)

	tmpDir, err := os.MkdirTemp("", "homeport-backup-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "homeport.db")
	if err := db.InitDB(dbPath); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("InitDB: %v", err)
	}
	if err := db.AddProfile("Markus", "markus"); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("AddProfile markus: %v", err)
	}
	if err := db.SetDefaultProfile("markus"); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("SetDefaultProfile: %v", err)
	}
	if err := db.AddProfile("Andrea", "andrea"); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("AddProfile andrea: %v", err)
	}

	cfg := &config.Config{
		DBPath:        dbPath,
		BackupDir:     filepath.Join(tmpDir, "backups"),
		BackupMaxKeep: 10,
	}
	srv := api.New(cfg)

	cleanup := func() {
		if db.DB != nil {
			db.DB.Close()
			db.DB = nil
		}
		os.RemoveAll(tmpDir)
		log.SetOutput(os.Stderr)
	}
	return srv, dbPath, cleanup
}

// multipartUpload builds an *http.Request with a multipart/form-data body
// containing a single file field with the given content.
func multipartUpload(t *testing.T, fieldName string, filename string, content []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	w.Close()
	req := httptest.NewRequest(http.MethodPost, "/manage/restore", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

// TestHandleBackupDownload_ValidSQLiteFile verifies that HandleBackupDownload
// streams a valid SQLite database file as a downloadable attachment.
func TestHandleBackupDownload_ValidSQLiteFile(t *testing.T) {
	srv, _, cleanup := setupBackupTest(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/manage/backup", nil)
	rr := httptest.NewRecorder()
	srv.HandleBackupDownload(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	cd := rr.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") {
		t.Errorf("expected attachment in Content-Disposition, got %q", cd)
	}
	if !strings.Contains(cd, ".db") {
		t.Errorf("expected .db filename in Content-Disposition, got %q", cd)
	}

	ct := rr.Header().Get("Content-Type")
	if ct != "application/x-sqlite3" {
		t.Errorf("expected Content-Type application/x-sqlite3, got %q", ct)
	}

	// SQLite files must start with the magic header "SQLite format 3\000"
	body := rr.Body.Bytes()
	if len(body) < 16 {
		t.Fatalf("response body too short (%d bytes) to be a SQLite file", len(body))
	}
	if string(body[:15]) != "SQLite format 3" {
		t.Errorf("response body does not start with SQLite magic header, got: %q", body[:16])
	}
}

// TestHandleRestore_Success verifies the full restore round-trip:
//   - handler accepts the multipart upload
//   - WAL is checkpointed and truncated before the atomic swap
//   - temp file is created in the same directory as the DB (no cross-device rename)
//   - db.ReinitDB opens the restored DB so subsequent queries reflect restored state
//   - handler redirects to /manage#backup on success
func TestHandleRestore_Success(t *testing.T) {
	srv, dbPath, cleanup := setupBackupTest(t)
	defer cleanup()

	// Seed the live DB with a category that belongs in the snapshot.
	if _, err := db.AddCategory("BeforeRestore", "blue"); err != nil {
		t.Fatalf("AddCategory BeforeRestore: %v", err)
	}

	// Take a snapshot *before* adding the second category.
	snapDir := filepath.Join(filepath.Dir(dbPath), "snap")
	snapshotPath, err := backup.CreateSnapshot(dbPath, snapDir)
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}
	snapshotData, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("ReadFile snapshot: %v", err)
	}

	// Add a category AFTER the snapshot — must be absent after restore.
	if _, err := db.AddCategory("AfterSnapshot", "red"); err != nil {
		t.Fatalf("AddCategory AfterSnapshot: %v", err)
	}

	req := multipartUpload(t, "file", "backup.db", snapshotData)
	rr := httptest.NewRecorder()
	srv.HandleRestore(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d: %s", rr.Code, rr.Body.String())
	}
	if loc := rr.Header().Get("Location"); loc != "/manage#backup" {
		t.Errorf("expected redirect to /manage#backup, got %q", loc)
	}

	// db.DB is now the restored connection — verify it reflects snapshot state.
	cats, err := db.GetCategoriesWithServices("")
	if err != nil {
		t.Fatalf("GetCategoriesWithServices after restore: %v", err)
	}
	catNames := make(map[string]bool)
	for _, c := range cats {
		catNames[c.Name] = true
	}
	if !catNames["BeforeRestore"] {
		t.Error("restored DB is missing 'BeforeRestore' category – snapshot data lost")
	}
	if catNames["AfterSnapshot"] {
		t.Error("restored DB still contains 'AfterSnapshot' – WAL not flushed before swap (#162)")
	}
}

// TestHandleRestore_InvalidFile verifies that uploading a non-SQLite file
// returns HTTP 400 Bad Request (backup.Validate rejects it).
func TestHandleRestore_InvalidFile(t *testing.T) {
	srv, _, cleanup := setupBackupTest(t)
	defer cleanup()

	req := multipartUpload(t, "file", "evil.db", []byte("this is definitely not a sqlite file"))
	rr := httptest.NewRecorder()
	srv.HandleRestore(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid file, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestHandleRestore_MissingFile verifies that a multipart request without
// the "file" field returns HTTP 400 Bad Request.
func TestHandleRestore_MissingFile(t *testing.T) {
	srv, _, cleanup := setupBackupTest(t)
	defer cleanup()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("other_field", "value")
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/manage/restore", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()
	srv.HandleRestore(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing file field, got %d", rr.Code)
	}
}
