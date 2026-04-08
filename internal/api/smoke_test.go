package api_test

// Template smoke tests: render each main handler with the REAL templates
// (assets.FS via InitTemplates), not the testdata stubs.
//
// These tests exist to catch one specific failure class: a handler passes a
// struct to the template that is missing a field the template references.
// Go's html/template aborts execution at that point — the HTTP response is
// truncated mid-stream with "Internal Server Error" injected into the body.
// Unit tests with stub templates cannot catch this; only real templates can.
//
// If you add a field to a template partial, add it to the corresponding
// handler data struct AND to the test assertions below.

import (
	"net/http/httptest"
	"net/http"
	"strings"
	"testing"

	"github.com/zk35-de/homeport/assets"
	"github.com/zk35-de/homeport/internal/api"
)

func setupSmoke(t *testing.T) (*api.Server, func()) {
	t.Helper()
	srv, cleanup := setupTest(t)
	srv.InitTemplates(assets.FS)
	return srv, cleanup
}

func assertSmoke(t *testing.T, rr *httptest.ResponseRecorder, wantPanels []string) {
	t.Helper()
	if rr.Code != http.StatusOK {
		t.Errorf("status %d, want 200\nbody: %s", rr.Code, rr.Body.String())
		return
	}
	body := rr.Body.String()
	if strings.Contains(body, "Internal Server Error") {
		t.Errorf("template execution failed mid-stream (Internal Server Error in body)\nbody tail: %s",
			body[max(0, len(body)-500):])
		return
	}
	for _, panel := range wantPanels {
		if !strings.Contains(body, panel) {
			t.Errorf("missing expected content %q in response", panel)
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// TestSmoke_HandleManage verifies that /manage renders all tab panels.
// A missing field in ManageData (like DiscoveryItems) causes template
// execution to abort after the category list — all subsequent panels
// disappear silently.
func TestSmoke_HandleManage(t *testing.T) {
	srv, cleanup := setupSmoke(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/manage", nil)
	rr := httptest.NewRecorder()
	srv.HandleManage(rr, req)

	assertSmoke(t, rr, []string{
		`id="panel-services"`,
		`id="discovery-sources"`,
		`id="analytics-link"`,
		`id="backup"`,
		`id="appearance"`,
		`id="user"`,
		// All tab buttons must be present
		`data-panel="panel-services"`,
		`data-panel="user"`,
	})
}

// TestSmoke_HandleIndex verifies that the index page renders without errors.
func TestSmoke_HandleIndex(t *testing.T) {
	srv, cleanup := setupSmoke(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	srv.HandleIndex(rr, req)

	assertSmoke(t, rr, []string{
		`<nav>`,
		`class="app-homeport"`,
	})
}

// TestSmoke_HandleAddCategory verifies that the HTMX category-list partial
// (returned after every category/service mutation) renders without errors.
// This partial embeds category_list.html which references .DiscoveryItems.
func TestSmoke_HandleAddCategory(t *testing.T) {
	srv, cleanup := setupSmoke(t)
	defer cleanup()

	req := httptest.NewRequest("POST", "/manage/category",
		strings.NewReader("name=SmokeTest&color=blue"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srv.HandleAddCategory(rr, req)

	assertSmoke(t, rr, []string{
		`id="sortable-categories"`,
		`SmokeTest`,
	})
}
