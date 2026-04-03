package api_test

import (
	"bytes"

	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"


	"github.com/go-chi/chi/v5"
	"github.com/zk35-de/homeport/internal/api"
	"github.com/zk35-de/homeport/internal/config"
	"github.com/zk35-de/homeport/internal/db"
)

// setupTestWithLogin extends setupTest to also initialize srv.LoginTmpl.
func setupTestWithLogin(t *testing.T) (*api.Server, func()) {
	t.Helper()
	srv, cleanup := setupTest(t)
	loginTmpl, err := template.ParseFS(testTemplateFS, "testdata/login.html")
	if err != nil {
		cleanup()
		t.Fatalf("Failed to parse login template: %v", err)
	}
	srv.LoginTmpl = loginTmpl
	return srv, cleanup
}


// --- Auth ---

func TestHandleLoginGET(t *testing.T) {
	srv, cleanup := setupTestWithLogin(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/login", nil)
	rr := httptest.NewRecorder()
	srv.HandleLogin(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "login") {
		t.Errorf("expected login page, got %s", rr.Body.String())
	}
}

func TestHandleLoginPOST_WrongPassword(t *testing.T) {
	srv, cleanup := setupTestWithLogin(t)
	defer cleanup()

	formData := url.Values{}
	formData.Set("profile", "markus")
	formData.Set("password", "wrongpassword")

	req := httptest.NewRequest("POST", "/login", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srv.HandleLogin(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 with error, got %d", rr.Code)
	}
	// renderLogin should be called with error message
	body := rr.Body.String()
	if !strings.Contains(body, "login") {
		t.Errorf("expected login page, got %s", body)
	}
}

func TestHandleLogout(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	req := httptest.NewRequest("POST", "/logout", nil)
	rr := httptest.NewRecorder()
	srv.HandleLogout(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}

// --- RequireAuth ---

func TestRequireAuth_Disabled(t *testing.T) {
	cfg := &config.Config{AuthEnabled: false}
	middleware := api.RequireAuth(cfg)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/manage", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 (auth disabled), got %d", rr.Code)
	}
}

func TestRequireAuth_Enabled_Redirect(t *testing.T) {
	cfg := &config.Config{AuthEnabled: true}
	middleware := api.RequireAuth(cfg)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/manage", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect (auth enabled, no session), got %d", rr.Code)
	}
}

func TestRequireAuth_Enabled_Whitelist(t *testing.T) {
	cfg := &config.Config{AuthEnabled: true}
	middleware := api.RequireAuth(cfg)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/login", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for /login whitelist, got %d", rr.Code)
	}
}

// --- ServiceRedirect ---

func TestHandleServiceRedirect(t *testing.T) {
	_, cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("Cat", "blue")
	cats, _ := db.GetCategoriesWithServices("")
	db.AddService(cats[0].ID, "MySvc", "http://my.service", "", "", "", false, []string{"markus"})
	catsWithSvc, _ := db.GetCategoriesWithServices("")
	svcID := catsWithSvc[0].Services[0].ID

	r := chi.NewRouter()
	r.Get("/r/{id}", api.HandleServiceRedirect)

	t.Run("valid redirect", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/r/%d?p=markus", svcID), nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusFound {
			t.Errorf("expected 302, got %d", rr.Code)
		}
		if rr.Header().Get("Location") != "http://my.service" {
			t.Errorf("unexpected Location: %s", rr.Header().Get("Location"))
		}
	})

	t.Run("unknown id", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/r/99999", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rr.Code)
		}
	})
}

// --- CategoryList / SetCategorySortMode ---

func TestHandleCategoryList(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("ListCat", "green")

	req := httptest.NewRequest("GET", "/category-list", nil)
	rr := httptest.NewRecorder()
	srv.HandleCategoryList(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "ListCat") {
		t.Errorf("expected ListCat in body, got %s", rr.Body.String())
	}
}

func TestHandleSetCategorySortMode(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("SortCat", "red")
	cats, _ := db.GetCategoriesWithServices("")
	catID := cats[0].ID

	r := chi.NewRouter()
	r.Post("/manage/category/{id}/sortmode/{mode}", srv.HandleSetCategorySortMode)

	req := httptest.NewRequest("POST", fmt.Sprintf("/manage/category/%d/sortmode/manual", catID), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// --- GetService / UpdateService ---

func TestHandleGetService(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("Cat", "blue")
	cats, _ := db.GetCategoriesWithServices("")
	db.AddService(cats[0].ID, "EditSvc", "http://edit.svc", "", "", "", false, []string{"markus"})
	catsWithSvc, _ := db.GetCategoriesWithServices("")
	svcID := catsWithSvc[0].Services[0].ID

	r := chi.NewRouter()
	r.Get("/manage/service/{id}", srv.HandleGetService)

	t.Run("found", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/manage/service/%d", svcID), nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "EditSvc") {
			t.Errorf("expected service name in body, got %s", rr.Body.String())
		}
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/manage/service/99999", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rr.Code)
		}
	})
}

func TestHandleUpdateService(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("Cat", "blue")
	cats, _ := db.GetCategoriesWithServices("")
	db.AddService(cats[0].ID, "OldName", "http://old.url", "", "", "", false, []string{"markus"})
	catsWithSvc, _ := db.GetCategoriesWithServices("")
	svcID := catsWithSvc[0].Services[0].ID

	r := chi.NewRouter()
	r.Post("/manage/service/{id}", srv.HandleUpdateService)

	formData := url.Values{}
	formData.Set("name", "NewName")
	formData.Set("url", "http://new.url")
	formData.Add("visibility", "markus")

	req := httptest.NewRequest("POST", fmt.Sprintf("/manage/service/%d", svcID), strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	catsAfter, _ := db.GetCategoriesWithServices("")
	if catsAfter[0].Services[0].Name != "NewName" {
		t.Errorf("service not updated, got %s", catsAfter[0].Services[0].Name)
	}
}

// --- GetCategory / UpdateCategory / UpdateCategorySpan ---

func TestHandleGetCategory(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("EditCat", "purple")
	cats, _ := db.GetCategoriesWithServices("")
	catID := cats[0].ID

	r := chi.NewRouter()
	r.Get("/manage/category/{id}", srv.HandleGetCategory)

	t.Run("found", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/manage/category/%d", catID), nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "EditCat") {
			t.Errorf("expected category name, got %s", rr.Body.String())
		}
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/manage/category/99999", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rr.Code)
		}
	})
}

func TestHandleUpdateCategory(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("OldCat", "red")
	cats, _ := db.GetCategoriesWithServices("")
	catID := cats[0].ID

	r := chi.NewRouter()
	r.Post("/manage/category/{id}", srv.HandleUpdateCategory)

	formData := url.Values{}
	formData.Set("name", "UpdatedCat")
	formData.Set("layout", "list")
	formData.Set("color", "blue")

	req := httptest.NewRequest("POST", fmt.Sprintf("/manage/category/%d", catID), strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestHandleUpdateCategorySpan(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("SpanCat", "blue")
	cats, _ := db.GetCategoriesWithServices("")
	catID := cats[0].ID

	r := chi.NewRouter()
	r.Post("/manage/category/{id}/span/{span}", srv.HandleUpdateCategorySpan)

	req := httptest.NewRequest("POST", fmt.Sprintf("/manage/category/%d/span/2", catID), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// --- ReorderCategories / ReorderServices ---

func TestHandleReorderCategories(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("CatA", "red")
	db.AddCategory("CatB", "blue")
	cats, _ := db.GetCategoriesWithServices("")

	items := []map[string]int{
		{"id": cats[0].ID, "sort_order": 1},
		{"id": cats[1].ID, "sort_order": 0},
	}
	body, _ := json.Marshal(items)

	req := httptest.NewRequest("POST", "/manage/categories/reorder", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.HandleReorderCategories(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rr.Code)
	}
}

func TestHandleReorderServices(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("Cat", "blue")
	cats, _ := db.GetCategoriesWithServices("")
	db.AddService(cats[0].ID, "Svc1", "url1", "", "", "", false, []string{"markus"})
	db.AddService(cats[0].ID, "Svc2", "url2", "", "", "", false, []string{"markus"})
	catsWithSvc, _ := db.GetCategoriesWithServices("")
	svcs := catsWithSvc[0].Services

	items := []map[string]int{
		{"id": svcs[0].ID, "sort_order": 1},
		{"id": svcs[1].ID, "sort_order": 0},
	}
	body, _ := json.Marshal(items)

	req := httptest.NewRequest("POST", "/manage/services/reorder", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.HandleReorderServices(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rr.Code)
	}
}

// --- Profiles ---

func TestHandleAddProfile(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	t.Run("success", func(t *testing.T) {
		formData := url.Values{}
		formData.Set("name", "Charlie")
		formData.Set("slug", "charlie")

		req := httptest.NewRequest("POST", "/manage/profiles", strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()
		srv.HandleAddProfile(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "Charlie") {
			t.Errorf("expected profile in list, got %s", rr.Body.String())
		}
	})

	t.Run("missing slug", func(t *testing.T) {
		formData := url.Values{}
		formData.Set("name", "MissingSlug")

		req := httptest.NewRequest("POST", "/manage/profiles", strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()
		srv.HandleAddProfile(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
	})
}

func TestHandleDeleteProfile(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	db.AddProfile("Temp", "temp")

	r := chi.NewRouter()
	r.Delete("/manage/profiles/{slug}", srv.HandleDeleteProfile)

	req := httptest.NewRequest("DELETE", "/manage/profiles/temp", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestHandleSetDefaultProfile(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	db.AddProfile("Extra", "extra")

	r := chi.NewRouter()
	r.Post("/manage/profiles/{slug}/default", srv.HandleSetDefaultProfile)

	req := httptest.NewRequest("POST", "/manage/profiles/extra/default", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// --- Preferences ---

func TestHandleGetPreferences(t *testing.T) {
	_, cleanup := setupTest(t)
	defer cleanup()

	t.Run("default profile", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/user/preferences", nil)
		rr := httptest.NewRecorder()
		api.HandleGetPreferences(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected JSON, got %s", ct)
		}
	})

	t.Run("specific profile", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/user/preferences?profile=markus", nil)
		rr := httptest.NewRecorder()
		api.HandleGetPreferences(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})
}

func TestHandleSetPreferences(t *testing.T) {
	_, cleanup := setupTest(t)
	defer cleanup()

	patch := map[string]string{
		"theme":        "light",
		"accent_color": "#ff0000",
	}
	body, _ := json.Marshal(patch)

	req := httptest.NewRequest("PATCH", "/api/user/preferences?profile=markus", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	api.HandleSetPreferences(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rr.Code)
	}

	// Verify change persisted
	prefs, _ := db.GetUserPreferences("markus")
	if prefs != nil && prefs.Theme != "light" {
		t.Errorf("expected theme=light, got %s", prefs.Theme)
	}
}

// --- Security Middleware ---

func TestSecurityHeaders(t *testing.T) {
	handler := api.SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("missing X-Content-Type-Options header")
	}
	if rr.Header().Get("X-Frame-Options") != "SAMEORIGIN" {
		t.Errorf("missing X-Frame-Options header")
	}
}

func TestCSRFMiddleware_SafeMethods(t *testing.T) {
	handler := api.CSRFMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, method := range []string{"GET", "HEAD", "OPTIONS"} {
		req := httptest.NewRequest(method, "/some/path", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("%s: expected 200, got %d", method, rr.Code)
		}
	}
}

func TestCSRFMiddleware_APIRouteNotExempt(t *testing.T) {
	// Bearer token auth was removed in #97 – state-changing /api/ requests
	// must now pass CSRF validation like any other POST/PATCH/DELETE.
	handler := api.CSRFMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("PATCH", "/api/user/preferences", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected /api/ state-changing request to require CSRF, got %d", rr.Code)
	}
}

func TestCSRFMiddleware_LoginExempt(t *testing.T) {
	handler := api.CSRFMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/login", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected /login POST to be exempt from CSRF, got %d", rr.Code)
	}
}

func TestCSRFMiddleware_MissingToken(t *testing.T) {
	handler := api.CSRFMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/manage/something", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 for missing CSRF token, got %d", rr.Code)
	}
}

func TestCSRFMiddleware_ValidToken(t *testing.T) {
	handler := api.CSRFMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	token := "test-csrf-token-1234"
	req := httptest.NewRequest("POST", "/manage/something", nil)
	req.AddCookie(&http.Cookie{Name: "hp_csrf", Value: token})
	req.Header.Set("X-CSRF-Token", token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 with valid CSRF token, got %d", rr.Code)
	}
}

func TestCSRFMiddleware_InvalidToken(t *testing.T) {
	handler := api.CSRFMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/manage/something", nil)
	req.AddCookie(&http.Cookie{Name: "hp_csrf", Value: "correct-token"})
	req.Header.Set("X-CSRF-Token", "wrong-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 for wrong CSRF token, got %d", rr.Code)
	}
}

// --- Pages ---

func TestHandleUpdatePage(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	pageID, _ := db.AddPage("markus", "OldPage", "📄")

	r := chi.NewRouter()
	r.Patch("/manage/page/{id}", srv.HandleUpdatePage)

	formData := url.Values{}
	formData.Set("name", "UpdatedPage")
	formData.Set("icon", "📝")
	formData.Set("profile", "markus")

	req := httptest.NewRequest("PATCH", fmt.Sprintf("/manage/page/%d", pageID), strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestHandleSortPage(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	db.AddPage("markus", "Page1", "📄")
	db.AddPage("markus", "Page2", "📄")
	pages, _ := db.GetPages("markus")

	r := chi.NewRouter()
	r.Post("/manage/sort/page/{id}/{direction}", srv.HandleSortPage)

	req := httptest.NewRequest("POST", fmt.Sprintf("/manage/sort/page/%d/down?profile=markus", pages[0].ID), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestHandleSetCategoryPage(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("Cat", "blue")
	cats, _ := db.GetCategoriesWithServices("")
	catID := cats[0].ID

	pageID, _ := db.AddPage("markus", "MyPage", "📄")

	r := chi.NewRouter()
	r.Post("/manage/category/{id}/page/{pageID}", srv.HandleSetCategoryPage)

	req := httptest.NewRequest("POST", fmt.Sprintf("/manage/category/%d/page/%d", catID, pageID), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// --- Discovery Sources ---

func TestHandleGetDiscoverySources(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/manage/discovery/sources", nil)
	rr := httptest.NewRecorder()
	srv.HandleGetDiscoverySources(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "discovery-sources") {
		t.Errorf("expected discovery-sources in body, got %s", rr.Body.String())
	}
}

func TestHandleAddDiscoverySource_Validation(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	t.Run("missing fields", func(t *testing.T) {
		formData := url.Values{}
		formData.Set("name", "TestSource")
		// missing type and url

		req := httptest.NewRequest("POST", "/manage/discovery/sources", strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()
		srv.HandleAddDiscoverySource(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for missing fields, got %d", rr.Code)
		}
	})

	t.Run("invalid url scheme", func(t *testing.T) {
		formData := url.Values{}
		formData.Set("type", "npm")
		formData.Set("name", "TestSource")
		formData.Set("url", "ftp://invalid.url")

		req := httptest.NewRequest("POST", "/manage/discovery/sources", strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()
		srv.HandleAddDiscoverySource(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid URL, got %d", rr.Code)
		}
	})
}

func TestHandleDeleteDiscoverySource_BadID(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	r := chi.NewRouter()
	r.Delete("/manage/discovery/sources/{id}", srv.HandleDeleteDiscoverySource)

	req := httptest.NewRequest("DELETE", "/manage/discovery/sources/not-a-number", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad ID, got %d", rr.Code)
	}
}

func TestHandleToggleDiscoverySource_BadID(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	r := chi.NewRouter()
	r.Post("/manage/discovery/sources/{id}/toggle", srv.HandleToggleDiscoverySource)

	req := httptest.NewRequest("POST", "/manage/discovery/sources/not-a-number/toggle", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad ID, got %d", rr.Code)
	}
}

func TestHandleDeleteDiscoverySource_Success(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	// Add a source directly via DB (no goroutine started)
	srcID, _ := db.AddDiscoverySource("npm", "TestSource", "http://localhost:19999", "", 3600)

	r := chi.NewRouter()
	r.Delete("/manage/discovery/sources/{id}", srv.HandleDeleteDiscoverySource)

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/manage/discovery/sources/%d", srcID), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for delete, got %d", rr.Code)
	}
}

func TestHandleToggleDiscoverySource_Success(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	// Add a disabled source so Reload doesn't start a goroutine
	srcID, _ := db.AddDiscoverySource("npm", "ToggleSource", "http://localhost:19999", "", 3600)
	// Disable it first to prevent goroutine start on toggle
	db.SetDiscoverySourceEnabled(int(srcID), false)

	r := chi.NewRouter()
	r.Post("/manage/discovery/sources/{id}/toggle", srv.HandleToggleDiscoverySource)

	req := httptest.NewRequest("POST", fmt.Sprintf("/manage/discovery/sources/%d/toggle", srcID), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for toggle, got %d", rr.Code)
	}
}

func TestHandleScanDiscoverySource_Success(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	srcID, _ := db.AddDiscoverySource("npm", "ScanSource", "http://localhost:19999", "", 3600)

	r := chi.NewRouter()
	r.Post("/manage/discovery/sources/{id}/scan", srv.HandleScanDiscoverySource)

	req := httptest.NewRequest("POST", fmt.Sprintf("/manage/discovery/sources/%d/scan", srcID), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204 for scan, got %d", rr.Code)
	}
}

func TestHandleScanDiscoverySource_BadID(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	r := chi.NewRouter()
	r.Post("/manage/discovery/sources/{id}/scan", srv.HandleScanDiscoverySource)

	req := httptest.NewRequest("POST", "/manage/discovery/sources/not-a-number/scan", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad ID, got %d", rr.Code)
	}
}

// --- Auth Management ---

func TestHandleManageAuth(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/manage/auth", nil)
	rr := httptest.NewRecorder()
	srv.HandleManageAuth(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "auth_list") {
		t.Errorf("expected auth_list in body, got %s", rr.Body.String())
	}
}

func TestHandleSetPassword(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	formData := url.Values{}
	formData.Set("profile", "markus")
	formData.Set("password", "testpassword123")

	req := httptest.NewRequest("POST", "/manage/auth/password", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srv.HandleSetPassword(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestHandleSetPassword_BadRequest(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	// Missing password
	formData := url.Values{}
	formData.Set("profile", "markus")

	req := httptest.NewRequest("POST", "/manage/auth/password", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srv.HandleSetPassword(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing password, got %d", rr.Code)
	}
}

func TestHandleDeletePassword(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	// Set a password first
	db.SetPassword("markus", "apassword")

	req := httptest.NewRequest("DELETE", "/manage/auth/password", strings.NewReader(url.Values{"profile": {"markus"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srv.HandleDeletePassword(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// --- Handle404 / Backup nil config ---

func TestHandle404(t *testing.T) {
	req := httptest.NewRequest("GET", "/nonexistent", nil)
	rr := httptest.NewRecorder()
	api.Handle404(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestHandleBackupDownload_NilConfig(t *testing.T) {
	srv := api.New(nil)
	req := httptest.NewRequest("GET", "/manage/backup", nil)
	rr := httptest.NewRecorder()
	srv.HandleBackupDownload(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 with nil config, got %d", rr.Code)
	}
}

func TestHandleRestore_NilConfig(t *testing.T) {
	srv := api.New(nil)
	req := httptest.NewRequest("POST", "/manage/restore", nil)
	rr := httptest.NewRecorder()
	srv.HandleRestore(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 with nil config, got %d", rr.Code)
	}
}

func TestCSRFToken(t *testing.T) {
	t.Run("with cookie", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "hp_csrf", Value: "my-token"})
		token := api.CSRFToken(req)
		if token != "my-token" {
			t.Errorf("expected my-token, got %q", token)
		}
	})

	t.Run("without cookie", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		token := api.CSRFToken(req)
		if token != "" {
			t.Errorf("expected empty token, got %q", token)
		}
	})
}

// --- ReorderCategories / ReorderServices bad JSON ---

func TestHandleReorderCategories_BadJSON(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	req := httptest.NewRequest("POST", "/manage/categories/reorder", strings.NewReader("not-json"))
	rr := httptest.NewRecorder()
	srv.HandleReorderCategories(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad JSON, got %d", rr.Code)
	}
}

func TestHandleReorderServices_BadJSON(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	req := httptest.NewRequest("POST", "/manage/services/reorder", strings.NewReader("not-json"))
	rr := httptest.NewRecorder()
	srv.HandleReorderServices(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad JSON, got %d", rr.Code)
	}
}

// --- HandleAddPage bad request ---

func TestHandleAddPage_BadRequest(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	formData := url.Values{}
	formData.Set("profile", "markus")
	// missing name

	req := httptest.NewRequest("POST", "/manage/page", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srv.HandleAddPage(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing name, got %d", rr.Code)
	}
}

// --- UpdateHub (SSE) ---

func TestUpdateHub_Broadcast_NoClients(t *testing.T) {
	hub := api.NewUpdateHub()
	// Should not panic or block with no clients
	hub.Broadcast(api.Message{Type: api.ServiceStatusMsg, Payload: "test"})
}
