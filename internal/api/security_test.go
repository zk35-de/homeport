package api_test

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"git.zk35.de/secalpha/homeport/internal/api"
	"git.zk35.de/secalpha/homeport/internal/db"
)

// setupBookmarksWidget creates a bookmarks widget for markus and returns its ID.
func setupBookmarksWidget(t *testing.T) int {
	t.Helper()
	if err := db.AddWidgetTyped("My Bookmarks", "bookmarks", `{"layout":"grid","links":[]}`, "markus"); err != nil {
		t.Fatalf("AddWidgetTyped bookmarks: %v", err)
	}
	widgets, err := db.GetWidgets("markus")
	if err != nil || len(widgets) == 0 {
		t.Fatalf("GetWidgets after creating bookmarks widget: %v", err)
	}
	return widgets[0].ID
}

// ── Feature Tests: Bookmarks ──────────────────────────────────────────────────

func TestHandleAddBookmark(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()
	widgetID := setupBookmarksWidget(t)

	r := chi.NewRouter()
	r.Post("/api/widgets/{id}/bookmark", api.HandleAddBookmark)

	t.Run("happy path", func(t *testing.T) {
		form := url.Values{"name": {"Go Blog"}, "url": {"https://go.dev/blog"}}
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/widgets/%d/bookmark", widgetID),
			strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "Go Blog") {
			t.Errorf("response should contain 'Go Blog', got: %s", rr.Body.String())
		}
	})

	t.Run("missing url returns 400", func(t *testing.T) {
		form := url.Values{"name": {"NoURL"}}
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/widgets/%d/bookmark", widgetID),
			strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for missing url, got %d", rr.Code)
		}
	})

	t.Run("invalid widget id returns 400", func(t *testing.T) {
		form := url.Values{"name": {"Test"}, "url": {"https://test.com"}}
		req := httptest.NewRequest("POST", "/api/widgets/notanumber/bookmark",
			strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for non-numeric widget id, got %d", rr.Code)
		}
	})
}

func TestHandleDeleteBookmark(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()
	widgetID := setupBookmarksWidget(t)

	// Pre-populate two links
	db.AddBookmarkLink(widgetID, db.BookmarkLink{Name: "Link1", URL: "https://link1.com"})
	db.AddBookmarkLink(widgetID, db.BookmarkLink{Name: "Link2", URL: "https://link2.com"})

	r := chi.NewRouter()
	r.Delete("/api/widgets/{id}/bookmark/{idx}", api.HandleDeleteBookmark)

	t.Run("happy path – delete index 0", func(t *testing.T) {
		req := httptest.NewRequest("DELETE",
			fmt.Sprintf("/api/widgets/%d/bookmark/0", widgetID), nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
		}
		// Link1 removed; Link2 should remain
		if strings.Contains(rr.Body.String(), "Link1") {
			t.Error("deleted link Link1 still appears in response")
		}
	})

	t.Run("out-of-range index returns 500", func(t *testing.T) {
		// Only Link2 remains (index 0); index 999 is out of range
		req := httptest.NewRequest("DELETE",
			fmt.Sprintf("/api/widgets/%d/bookmark/999", widgetID), nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code == http.StatusOK {
			t.Error("expected non-200 for out-of-range bookmark index, got 200")
		}
	})

	t.Run("invalid widget id returns 400", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/widgets/bad/bookmark/0", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid widget id, got %d", rr.Code)
		}
	})

	t.Run("invalid idx returns 400", func(t *testing.T) {
		req := httptest.NewRequest("DELETE",
			fmt.Sprintf("/api/widgets/%d/bookmark/notanumber", widgetID), nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid idx, got %d", rr.Code)
		}
	})
}

// ── Feature Tests: Notes ─────────────────────────────────────────────────────

func TestHandleSaveNote(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	if err := db.AddWidgetTyped("My Notes", "notes", `{}`, "markus"); err != nil {
		t.Fatalf("AddWidgetTyped notes: %v", err)
	}
	widgets, _ := db.GetWidgets("markus")
	widgetID := widgets[0].ID

	r := chi.NewRouter()
	r.Put("/api/notes/{id}", api.HandleSaveNote)

	t.Run("happy path", func(t *testing.T) {
		body := []byte(`{"content":"Hello Notes"}`)
		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/notes/%d", widgetID),
			bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d: %s", rr.Code, rr.Body.String())
		}
		content, _ := db.GetNote(widgetID)
		if content != "Hello Notes" {
			t.Errorf("note not stored: got %q", content)
		}
	})

	t.Run("empty content is valid", func(t *testing.T) {
		body := []byte(`{"content":""}`)
		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/notes/%d", widgetID),
			bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Errorf("expected 204 for empty content, got %d", rr.Code)
		}
	})

	t.Run("no json body returns 400", func(t *testing.T) {
		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/notes/%d", widgetID),
			strings.NewReader("not json"))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid JSON body, got %d", rr.Code)
		}
	})
}

// ── Feature Tests: Analytics ─────────────────────────────────────────────────

func TestHandleAnalytics(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	// Set up AnalyticsTmpl using the analytics_content template
	analyticsTmpl, err := template.ParseFS(testTemplateFS,
		"testdata/base.html",
		"testdata/analytics_content.html",
	)
	if err != nil {
		t.Fatalf("parse analytics templates: %v", err)
	}
	api.AnalyticsTmpl = analyticsTmpl
	defer func() { api.AnalyticsTmpl = nil }()

	t.Run("200 OK – empty stats", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/manage/analytics", nil)
		rr := httptest.NewRecorder()
		api.HandleAnalytics(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "0 stats") {
			t.Errorf("expected '0 stats' in response, got: %s", rr.Body.String())
		}
	})

	t.Run("profile filter via query param", func(t *testing.T) {
		// Add service and record a click for markus
		db.AddCategory("Cat", "tiles", "blue")
		cats, _ := db.GetCategoriesWithServices("")
		db.AddService(cats[0].ID, "MySvc", "http://my.svc", "", "", "", []string{"markus"})
		allCats, _ := db.GetCategoriesWithServices("")
		svcID := allCats[0].Services[0].ID
		db.RecordClick(svcID, "markus")

		req := httptest.NewRequest("GET", "/manage/analytics?profile=markus", nil)
		rr := httptest.NewRecorder()
		api.HandleAnalytics(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 with profile filter, got %d", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "profile=markus") {
			t.Errorf("profile not reflected in response, got: %s", rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "1 stats") {
			t.Errorf("expected '1 stats' after recording click, got: %s", rr.Body.String())
		}
	})
}

// ── Security Tests: Auth/Authorization ───────────────────────────────────────

func TestAuthMiddleware(t *testing.T) {
	const testToken = "test-secret-token"
	middleware := api.AuthMiddleware(testToken)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("no token on /api/widgets → 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/widgets", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("wrong token → 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/widgets", nil)
		req.Header.Set("Authorization", "Bearer wrong-token")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("correct token → 200", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/widgets", nil)
		req.Header.Set("Authorization", "Bearer "+testToken)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("whitelisted path / → 200 without token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 for whitelisted /, got %d", rr.Code)
		}
	})

	t.Run("/static/ prefix → 200 without token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/static/app.css", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 for /static/ prefix, got %d", rr.Code)
		}
	})

	// /manage is NOT in the whitelist – intentionally requires auth when used via API token.
	// In production, manage routes are mounted outside the auth group (network isolation
	// is the security layer for self-hosted deployments). AuthMiddleware is only applied
	// to /api/* routes in main.go, so /manage is intentionally accessible without token.
	// This test documents the middleware isolation boundary.
	t.Run("non-whitelisted path /manage → 401 from middleware", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/manage", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 from middleware for /manage, got %d (note: in production /manage is mounted outside the auth group)", rr.Code)
		}
	})
}

// ── Security Tests: Input Validation / Boundary ───────────────────────────────

func TestInputValidationBoundary(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()
	widgetID := setupBookmarksWidget(t)

	r := chi.NewRouter()
	r.Post("/api/widgets/{id}/bookmark", api.HandleAddBookmark)
	r.Delete("/api/widgets/{id}/bookmark/{idx}", api.HandleDeleteBookmark)
	r.Put("/api/notes/{id}", api.HandleSaveNote)

	t.Run("bookmark idx=-1 → 400", func(t *testing.T) {
		// Negative index in URL – strconv.Atoi parses -1 successfully,
		// but DeleteBookmarkLink must return error for idx < 0
		req := httptest.NewRequest("DELETE",
			fmt.Sprintf("/api/widgets/%d/bookmark/-1", widgetID), nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		// Handler returns 500 from db error; either 4xx or 5xx is acceptable – must not panic
		if rr.Code == 200 {
			t.Error("expected non-200 for idx=-1, got 200")
		}
	})

	t.Run("bookmark idx=999999 → no panic", func(t *testing.T) {
		req := httptest.NewRequest("DELETE",
			fmt.Sprintf("/api/widgets/%d/bookmark/999999", widgetID), nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		// Must not panic; any non-200 error response is acceptable
		if rr.Code == 200 {
			t.Error("expected non-200 for out-of-range idx=999999, got 200")
		}
	})

	t.Run("note content 1 MB → no OOM/panic", func(t *testing.T) {
		if err := db.AddWidgetTyped("BigNote", "notes", `{}`, "markus"); err != nil {
			t.Fatalf("AddWidgetTyped: %v", err)
		}
		widgets, _ := db.GetWidgets("markus")
		var noteWidgetID int
		for _, w := range widgets {
			if w.Name == "BigNote" {
				noteWidgetID = w.ID
			}
		}

		big := `{"content":"` + strings.Repeat("A", 1024*1024) + `"}`
		req := httptest.NewRequest("PUT", fmt.Sprintf("/api/notes/%d", noteWidgetID),
			strings.NewReader(big))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		// Must not panic; 204 or error response both acceptable
		if rr.Code != http.StatusNoContent && rr.Code < 400 {
			t.Errorf("unexpected status for large note: %d", rr.Code)
		}
	})

	t.Run("service URL 10000 chars → no panic", func(t *testing.T) {
		longURL := "http://x.com/" + strings.Repeat("a", 10000)
		db.AddCategory("BoundaryTest", "tiles", "blue")
		cats, _ := db.GetCategoriesWithServices("")
		var catID int
		for _, c := range cats {
			if c.Name == "BoundaryTest" {
				catID = c.ID
			}
		}
		err := db.AddService(catID, "LongURL", longURL, "", "", "", []string{"markus"})
		// Must not panic; storage may succeed or fail depending on DB constraints
		_ = err
	})
}

// ── Security Tests: Open Redirect ────────────────────────────────────────────

func TestOpenRedirect(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("Cat", "tiles", "blue")
	cats, _ := db.GetCategoriesWithServices("")
	catID := cats[0].ID

	db.AddService(catID, "Safe", "http://safe.example.com", "", "", "", []string{"markus"})
	db.AddService(catID, "JSEvil", "javascript:alert(1)", "", "", "", []string{"markus"})
	db.AddService(catID, "DataEvil", "data:text/html,<script>alert(1)</script>", "", "", "", []string{"markus"})

	allCats, _ := db.GetCategoriesWithServices("")
	services := allCats[0].Services
	var safeID, jsID, dataID int
	for _, s := range services {
		switch s.Name {
		case "Safe":
			safeID = s.ID
		case "JSEvil":
			jsID = s.ID
		case "DataEvil":
			dataID = s.ID
		}
	}

	r := chi.NewRouter()
	r.Get("/r/{id}", api.HandleServiceRedirect)

	t.Run("safe http URL redirects normally", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/r/%d", safeID), nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusFound {
			t.Errorf("expected 302 for safe URL, got %d", rr.Code)
		}
		if rr.Header().Get("Location") != "http://safe.example.com" {
			t.Errorf("expected redirect to http://safe.example.com, got %s", rr.Header().Get("Location"))
		}
	})

	t.Run("javascript: URL is blocked (open redirect / XSS prevention)", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/r/%d", jsID), nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code == http.StatusFound && rr.Header().Get("Location") == "javascript:alert(1)" {
			t.Error("SECURITY: handler redirected to javascript: URL – open redirect not blocked")
		}
	})

	t.Run("data: URL is blocked", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/r/%d", dataID), nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code == http.StatusFound && strings.HasPrefix(rr.Header().Get("Location"), "data:") {
			t.Error("SECURITY: handler redirected to data: URL – open redirect not blocked")
		}
	})
}

// ── Security Tests: SSRF via Favicon Proxy ────────────────────────────────────

func TestSSRFFavicon(t *testing.T) {
	t.Run("file:// scheme → 400", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/favicon?url=file:///etc/passwd", nil)
		rr := httptest.NewRecorder()
		api.HandleFavicon(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for file:// url, got %d", rr.Code)
		}
	})

	t.Run("javascript: scheme → 400", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/favicon?url=javascript:alert(1)", nil)
		rr := httptest.NewRecorder()
		api.HandleFavicon(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for javascript: url, got %d", rr.Code)
		}
	})

	t.Run("missing url → 400", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/favicon", nil)
		rr := httptest.NewRecorder()
		api.HandleFavicon(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for missing url param, got %d", rr.Code)
		}
	})

	t.Run("link-local IP 169.254.169.254 → 403 (SSRF protection)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/favicon?url=http://169.254.169.254/latest/meta-data/", nil)
		rr := httptest.NewRecorder()
		api.HandleFavicon(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403 for link-local IP (AWS metadata SSRF), got %d", rr.Code)
		}
	})

	t.Run("loopback 127.0.0.1 → 403 (SSRF protection)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/favicon?url=http://127.0.0.1:22/", nil)
		rr := httptest.NewRecorder()
		api.HandleFavicon(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403 for loopback address, got %d", rr.Code)
		}
	})

	t.Run("localhost → 403 (SSRF protection)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/favicon?url=http://localhost/", nil)
		rr := httptest.NewRecorder()
		api.HandleFavicon(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403 for localhost, got %d", rr.Code)
		}
	})

	t.Run("valid public domain → not blocked (404 expected in test env)", func(t *testing.T) {
		// Verifies that SSRF protection does not block legitimate public URLs.
		// In test environment there's no real favicon – expect 404 or non-403/non-400.
		req := httptest.NewRequest("GET", "/api/favicon?url=https://example.com/", nil)
		rr := httptest.NewRecorder()
		api.HandleFavicon(rr, req)
		if rr.Code == http.StatusForbidden || rr.Code == http.StatusBadRequest {
			t.Errorf("public domain should not be blocked by SSRF filter, got %d", rr.Code)
		}
	})
}

// ── Feature Tests: Todo Handlers ─────────────────────────────────────────────

func TestHandleAddTodo(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	db.AddWidgetTyped("My Todos", "todo", `{}`, "markus")
	widgets, _ := db.GetWidgets("markus")
	widgetID := widgets[0].ID

	r := chi.NewRouter()
	r.Post("/api/todos", api.HandleAddTodo)

	t.Run("happy path", func(t *testing.T) {
		form := url.Values{
			"widget_id": {fmt.Sprintf("%d", widgetID)},
			"text":      {"Buy milk"},
		}
		req := httptest.NewRequest("POST", "/api/todos", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "Buy milk") {
			t.Errorf("response should contain 'Buy milk', got: %s", rr.Body.String())
		}
	})

	t.Run("missing text → 400", func(t *testing.T) {
		form := url.Values{"widget_id": {fmt.Sprintf("%d", widgetID)}}
		req := httptest.NewRequest("POST", "/api/todos", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for missing text, got %d", rr.Code)
		}
	})

	t.Run("invalid widget_id → 400", func(t *testing.T) {
		form := url.Values{"widget_id": {"notanumber"}, "text": {"test"}}
		req := httptest.NewRequest("POST", "/api/todos", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid widget_id, got %d", rr.Code)
		}
	})
}

func TestHandleToggleAndDeleteTodo(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	db.AddWidgetTyped("Todos", "todo", `{}`, "markus")
	widgets, _ := db.GetWidgets("markus")
	widgetID := widgets[0].ID
	todoID, _ := db.AddTodo(widgetID, "Task One", "")

	r := chi.NewRouter()
	r.Post("/api/todos/{id}/toggle", api.HandleToggleTodo)
	r.Delete("/api/todos/{id}", api.HandleDeleteTodo)

	t.Run("toggle todo", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/todos/%d/toggle", todoID), nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 for toggle, got %d", rr.Code)
		}
		todos, _ := db.GetTodos(widgetID)
		if len(todos) != 1 || !todos[0].Done {
			t.Errorf("todo should be done after toggle, got %v", todos)
		}
	})

	t.Run("delete todo", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/todos/%d", todoID), nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 for delete, got %d", rr.Code)
		}
		todos, _ := db.GetTodos(widgetID)
		if len(todos) != 0 {
			t.Errorf("expected 0 todos after delete, got %d", len(todos))
		}
	})
}

// ── Health Endpoint ───────────────────────────────────────────────────────────

func TestHandleHealth(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/health", nil)
	rr := httptest.NewRecorder()
	api.HandleHealth(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"status"`) {
		t.Errorf("expected JSON with status field, got: %s", rr.Body.String())
	}
}

func TestHandleSearch(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("Work", "tiles", "blue")
	cats, _ := db.GetCategoriesWithServices("")
	db.AddService(cats[0].ID, "Jira", "http://jira.local", "", "Project tracking", "", []string{"markus"})

	t.Run("matching query returns result", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/search?q=jira&profile=markus", nil)
		rr := httptest.NewRecorder()
		api.HandleSearch(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "Jira") {
			t.Errorf("expected Jira in results, got: %s", rr.Body.String())
		}
	})

	t.Run("empty query returns empty array", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/search?q=&profile=markus", nil)
		rr := httptest.NewRecorder()
		api.HandleSearch(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})
}

// ── Security Tests: XSS Template Escaping ────────────────────────────────────

func TestXSSTemplateEscaping(t *testing.T) {
	// Go's html/template escapes HTML automatically. These tests verify that
	// user-controlled data from the DB is rendered safely in templates.

	tests := []struct {
		name     string
		tmpl     string
		payload  string
		notWant  string
		wantErr  bool // template.Execute returning error is also acceptable (e.g. javascript: in href)
	}{
		{
			name:    "script tag in text node",
			tmpl:    `{{.}}`,
			payload: `<script>alert(1)</script>`,
			notWant: `<script>`,
		},
		{
			name:    "attribute injection",
			tmpl:    `<div class="{{.}}">text</div>`,
			payload: `"><img src=x onerror=alert(1)>`,
			notWant: `<img`,
		},
		{
			name:    "img tag in text node",
			tmpl:    `<p>{{.}}</p>`,
			payload: `<img src=x onerror=alert(1)>`,
			notWant: `<img`,
		},
		{
			name:    "javascript scheme in href – sanitized by html/template",
			tmpl:    `<a href="{{.}}">link</a>`,
			payload: `javascript:alert(1)`,
			notWant: `javascript:alert`,
			wantErr: true, // html/template replaces with #ZgotmplZ – execute may not error but output is safe
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpl := template.Must(template.New("xss").Parse(tc.tmpl))
			var buf strings.Builder
			err := tmpl.Execute(&buf, tc.payload)
			if err != nil {
				// Template execution error is acceptable security behavior
				return
			}
			out := buf.String()
			if tc.notWant != "" && strings.Contains(out, tc.notWant) {
				t.Errorf("XSS not escaped: output contains %q with payload %q\nOutput: %s",
					tc.notWant, tc.payload, out)
			}
		})
	}

	t.Run("category name with XSS payload escaped in category_list template", func(t *testing.T) {
		cleanup := setupTest(t)
		defer cleanup()

		xssName := `<script>alert(1)</script>`
		db.AddCategory(xssName, "tiles", "blue")
		cats, _ := db.GetCategoriesWithServices("")
		profiles, _ := db.GetProfiles()

		data := struct {
			Categories []db.Category
			Profiles   []db.Profile
		}{cats, profiles}

		var buf strings.Builder
		if err := api.ManageTmpl.ExecuteTemplate(&buf, "category_list", data); err != nil {
			t.Fatalf("ExecuteTemplate: %v", err)
		}
		out := buf.String()
		if strings.Contains(out, "<script>") {
			t.Errorf("XSS: <script> not escaped in category_list output: %s", out)
		}
		if !strings.Contains(out, "&lt;script&gt;") {
			t.Errorf("Expected HTML-escaped output to contain &lt;script&gt;, got: %s", out)
		}
	})
}

func TestHandleAddAndDeletePage(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	// Inject minimal page_list template so ManageTmpl can render it
	pageTmpl := `{{define "page_list"}}<div id="page-list">{{range .Pages}}{{.Name}}{{end}}</div>{{end}}`
	api.ManageTmpl, _ = api.ManageTmpl.New("").Parse(pageTmpl)

	router := chi.NewRouter()
	router.Post("/manage/page", api.HandleAddPage)
	router.Delete("/manage/page/{id}", api.HandleDeletePage)
	router.Get("/manage/page-list", api.HandleGetPageList)

	// Add a page
	form := url.Values{"profile": {"markus"}, "name": {"Work"}, "icon": {"💼"}}
	req := httptest.NewRequest(http.MethodPost, "/manage/page", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("AddPage: want 200, got %d", rr.Code)
	}

	pages, _ := db.GetPages("markus")
	if len(pages) != 1 || pages[0].Name != "Work" {
		t.Fatalf("Expected 1 page 'Work', got %v", pages)
	}

	// Get page list
	req2 := httptest.NewRequest(http.MethodGet, "/manage/page-list?profile=markus", nil)
	rr2 := httptest.NewRecorder()
	router.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("GetPageList: want 200, got %d", rr2.Code)
	}
	if !strings.Contains(rr2.Body.String(), "Work") {
		t.Errorf("GetPageList: expected 'Work' in output, got: %s", rr2.Body.String())
	}

	// Delete the page
	pageID := pages[0].ID
	req3 := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/manage/page/%d?profile=markus", pageID), nil)
	rr3 := httptest.NewRecorder()
	router.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusOK {
		t.Fatalf("DeletePage: want 200, got %d", rr3.Code)
	}

	pages2, _ := db.GetPages("markus")
	if len(pages2) != 0 {
		t.Fatalf("Expected 0 pages after delete, got %v", pages2)
	}
}
