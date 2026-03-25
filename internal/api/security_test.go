package api_test

import (
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
		db.AddCategory("Cat", "blue")
		cats, _ := db.GetCategoriesWithServices("")
		db.AddService(cats[0].ID, "MySvc", "http://my.svc", "", "", "", false, []string{"markus"})
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


// ── Security Tests: Input Validation / Boundary ───────────────────────────────

func TestInputValidationBoundary(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	t.Run("service URL 10000 chars → no panic", func(t *testing.T) {
		longURL := "http://x.com/" + strings.Repeat("a", 10000)
		db.AddCategory("BoundaryTest", "blue")
		cats, _ := db.GetCategoriesWithServices("")
		var catID int
		for _, c := range cats {
			if c.Name == "BoundaryTest" {
				catID = c.ID
			}
		}
		err := db.AddService(catID, "LongURL", longURL, "", "", "", false, []string{"markus"})
		// Must not panic; storage may succeed or fail depending on DB constraints
		_ = err
	})
}

// ── Security Tests: Open Redirect ────────────────────────────────────────────

func TestOpenRedirect(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("Cat", "blue")
	cats, _ := db.GetCategoriesWithServices("")
	catID := cats[0].ID

	db.AddService(catID, "Safe", "http://safe.example.com", "", "", "", false, []string{"markus"})
	db.AddService(catID, "JSEvil", "javascript:alert(1)", "", "", "", false, []string{"markus"})
	db.AddService(catID, "DataEvil", "data:text/html,<script>alert(1)</script>", "", "", "", false, []string{"markus"})

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

	// Private/loopback IPs are intentionally allowed (homelab use case – services on
	// 192.168.x, 10.x, 127.x are valid favicon targets). Only scheme is validated.
	t.Run("link-local IP 169.254.169.254 → not blocked (homelab)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/favicon?url=http://169.254.169.254/", nil)
		rr := httptest.NewRecorder()
		api.HandleFavicon(rr, req)
		if rr.Code == http.StatusBadRequest {
			t.Errorf("valid http:// URL should not be rejected with 400, got %d", rr.Code)
		}
	})

	t.Run("loopback 127.0.0.1 → not blocked (homelab)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/favicon?url=http://127.0.0.1:8080/", nil)
		rr := httptest.NewRecorder()
		api.HandleFavicon(rr, req)
		if rr.Code == http.StatusBadRequest {
			t.Errorf("valid http:// URL should not be rejected with 400, got %d", rr.Code)
		}
	})

	t.Run("non-http scheme → 400 (scheme validation)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/favicon?url=file:///etc/passwd", nil)
		rr := httptest.NewRecorder()
		api.HandleFavicon(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for non-http scheme, got %d", rr.Code)
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

	db.AddCategory("Work", "blue")
	cats, _ := db.GetCategoriesWithServices("")
	db.AddService(cats[0].ID, "Jira", "http://jira.local", "", "Project tracking", "", false, []string{"markus"})

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
		db.AddCategory(xssName, "blue")
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
