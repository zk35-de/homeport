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

// TestHandleSetCategorySortMode_ServicesPreserved reproduces #152:
// after toggling sort mode the category list must still contain the service name.
func TestHandleSetCategorySortMode_ServicesPreserved(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("SortCat2", "blue")
	cats, _ := db.GetCategoriesWithServices("")
	catID := cats[0].ID
	db.AddService(catID, "MySvc", "http://my.svc", "", "", "", false, []string{"markus"})

	r := chi.NewRouter()
	r.Post("/manage/category/{id}/sortmode/{mode}", srv.HandleSetCategorySortMode)

	req := httptest.NewRequest("POST", fmt.Sprintf("/manage/category/%d/sortmode/usage", catID), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	// Testdata stub renders "CategoryName:NumServices@Color" – verify 1 service present.
	if !strings.Contains(body, "SortCat2:1@") {
		t.Errorf("#152 regression: category list empty after sortmode toggle (expected 1 service). Body: %s", body)
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

func TestCSRFMiddleware_LogoutExempt(t *testing.T) {
	handler := api.CSRFMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/logout", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected /logout POST to be exempt from CSRF, got %d", rr.Code)
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

// TestHandleAddDiscoverySource_SavesToDB verifies the success path of
// HandleAddDiscoverySource: a valid source is persisted in the database.
func TestHandleAddDiscoverySource_SavesToDB(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	formData := url.Values{}
	formData.Set("type", "npm")
	formData.Set("name", "My Docker Host")
	formData.Set("url", "http://localhost:2375")
	formData.Set("interval", "120")

	req := httptest.NewRequest("POST", "/manage/discovery/sources", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srv.HandleAddDiscoverySource(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	sources, err := db.GetDiscoverySources()
	if err != nil {
		t.Fatalf("GetDiscoverySources: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 discovery source in DB, got %d", len(sources))
	}
	if sources[0].Name != "My Docker Host" {
		t.Errorf("expected name 'My Docker Host', got %q", sources[0].Name)
	}
	if sources[0].URL != "http://localhost:2375" {
		t.Errorf("expected URL 'http://localhost:2375', got %q", sources[0].URL)
	}
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

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for scan, got %d", rr.Code)
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

// TestHandleSetPassword_ChangesDBState verifies that HandleSetPassword stores
// a verifiable bcrypt hash in the database (not just returns 200).
func TestHandleSetPassword_ChangesDBState(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	formData := url.Values{}
	formData.Set("profile", "markus")
	formData.Set("password", "securepass123")

	req := httptest.NewRequest("POST", "/manage/auth/password", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srv.HandleSetPassword(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !db.CheckPassword("markus", "securepass123") {
		t.Error("CheckPassword returned false after HandleSetPassword – hash not stored")
	}
	if db.CheckPassword("markus", "wrongpassword") {
		t.Error("CheckPassword returned true for wrong password – no validation")
	}
}

func TestHandleDeletePassword(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	// Set a password first
	db.SetPassword("markus", "apassword")

	// HTMX sends hx-delete form values as URL query parameters, not in the body.
	// Go's ParseForm only reads the body for POST/PUT/PATCH, not DELETE.
	req := httptest.NewRequest("DELETE", "/manage/auth/password?profile=markus", nil)
	rr := httptest.NewRecorder()
	srv.HandleDeletePassword(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// TestHandleDeletePassword_ChangesDBState verifies that HandleDeletePassword
// removes the password entry from the database.
func TestHandleDeletePassword_ChangesDBState(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	// Pre-condition: set a password so there is something to delete.
	if err := db.SetPassword("markus", "willbedeleted"); err != nil {
		t.Fatalf("SetPassword precondition: %v", err)
	}
	if !db.CheckPassword("markus", "willbedeleted") {
		t.Fatal("precondition failed: CheckPassword returned false before delete")
	}

	// HTMX sends hx-delete form values as URL query parameters, not in the body.
	req := httptest.NewRequest("DELETE", "/manage/auth/password?profile=markus", nil)
	rr := httptest.NewRecorder()
	srv.HandleDeletePassword(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if db.CheckPassword("markus", "willbedeleted") {
		t.Error("CheckPassword returned true after delete – password not removed")
	}
	auth, err := db.GetUserAuth("markus")
	if err != nil {
		t.Fatalf("GetUserAuth after delete: %v", err)
	}
	if auth != nil {
		t.Errorf("expected nil auth entry after delete, got %+v", auth)
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

// --- Manage: Category Visibility (#141 regression) ---

// TestHandleCategoryOptions_ReturnsOptions verifies that GET /manage/category-options
// returns an <option> element for each existing category.
// Regression test for #141 (categories not visible in right panel of /manage).
func TestHandleCategoryOptions_ReturnsOptions(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("Homelab", "blue")
	db.AddCategory("Media", "purple")

	req := httptest.NewRequest("GET", "/manage/category-options", nil)
	rr := httptest.NewRecorder()
	srv.HandleCategoryOptions(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Homelab") {
		t.Errorf("expected category 'Homelab' in options, got: %s", body)
	}
	if !strings.Contains(body, "Media") {
		t.Errorf("expected category 'Media' in options, got: %s", body)
	}
	if !strings.Contains(body, "<option") {
		t.Errorf("expected <option> elements, got: %s", body)
	}
}

// TestHandleCategoryOptions_EmptyWhenNoCategories verifies that the endpoint
// returns an empty body (no options) when there are no categories.
func TestHandleCategoryOptions_EmptyWhenNoCategories(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/manage/category-options", nil)
	rr := httptest.NewRecorder()
	srv.HandleCategoryOptions(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "<option") {
		t.Errorf("expected no <option> elements with empty DB, got: %s", rr.Body.String())
	}
}

// TestHandleManage_PassesCategoryData verifies that HandleManage passes all
// categories to the template. If Categories is empty, the right panel in
// /manage would show nothing (#141).
func TestHandleManage_PassesCategoryData(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("Servers", "red")
	db.AddCategory("Tools", "green")

	req := httptest.NewRequest("GET", "/manage", nil)
	rr := httptest.NewRecorder()
	srv.HandleManage(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	// The stub template renders: Manage: <cat1> <cat2> and the category_list partial
	if !strings.Contains(body, "Servers") {
		t.Errorf("expected category 'Servers' in manage output, got: %s", body)
	}
	if !strings.Contains(body, "Tools") {
		t.Errorf("expected category 'Tools' in manage output, got: %s", body)
	}
}

// TestHandleCategoryOptions_AfterAddCategory verifies that a newly added category
// immediately appears in /manage/category-options.
// This is the core regression: after adding a category, the right panel dropdown
// must reflect it without page reload.
func TestHandleCategoryOptions_AfterAddCategory(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	// Initially empty
	req := httptest.NewRequest("GET", "/manage/category-options", nil)
	rr := httptest.NewRecorder()
	srv.HandleCategoryOptions(rr, req)
	if strings.Contains(rr.Body.String(), "<option") {
		t.Fatal("precondition failed: expected no options before any category exists")
	}

	// Add category via handler
	form := url.Values{}
	form.Set("name", "NAS")
	form.Set("color", "orange")
	form.Set("layout", "tiles")
	addReq := httptest.NewRequest("POST", "/manage/category", strings.NewReader(form.Encode()))
	addReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addRr := httptest.NewRecorder()
	srv.HandleAddCategory(addRr, addReq)
	if addRr.Code != http.StatusOK {
		t.Fatalf("HandleAddCategory failed: %d", addRr.Code)
	}

	// Now category-options must contain the new category
	req2 := httptest.NewRequest("GET", "/manage/category-options", nil)
	rr2 := httptest.NewRecorder()
	srv.HandleCategoryOptions(rr2, req2)
	body := rr2.Body.String()
	if !strings.Contains(body, "NAS") {
		t.Errorf("newly added category 'NAS' not in /manage/category-options after add – right panel regression (#141). Got: %s", body)
	}
}

// --- Profile Options (#150) ---

// TestHandleProfileOptions_ReturnsOptions verifies that GET /manage/profile-options
// returns <option> elements for all existing profiles.
func TestHandleProfileOptions_ReturnsOptions(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/manage/profile-options", nil)
	rr := httptest.NewRecorder()
	srv.HandleProfileOptions(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	// setupTest creates markus/mgi/ako profiles
	if !strings.Contains(body, "markus") {
		t.Errorf("expected markus in profile-options, got: %s", body)
	}
}

// TestHandleProfileOptions_AfterAddProfile verifies that a newly added profile
// immediately appears in /manage/profile-options (#150).
func TestHandleProfileOptions_AfterAddProfile(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	// Verify it's not there yet
	req := httptest.NewRequest("GET", "/manage/profile-options", nil)
	rr := httptest.NewRecorder()
	srv.HandleProfileOptions(rr, req)
	if strings.Contains(rr.Body.String(), "newuser") {
		t.Fatal("precondition failed: newuser should not exist yet")
	}

	// Add profile via handler
	form := url.Values{}
	form.Set("name", "New User")
	form.Set("slug", "newuser")
	addReq := httptest.NewRequest("POST", "/manage/profile", strings.NewReader(form.Encode()))
	addReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addRr := httptest.NewRecorder()
	srv.HandleAddProfile(addRr, addReq)
	if addRr.Code != http.StatusOK {
		t.Fatalf("HandleAddProfile failed: %d %s", addRr.Code, addRr.Body.String())
	}

	// Now profile-options must include the new profile
	req2 := httptest.NewRequest("GET", "/manage/profile-options", nil)
	rr2 := httptest.NewRecorder()
	srv.HandleProfileOptions(rr2, req2)
	body := rr2.Body.String()
	if !strings.Contains(body, "newuser") {
		t.Errorf("newly added profile 'newuser' not in /manage/profile-options (#150). Got: %s", body)
	}
}

// --- UpdateHub (SSE) ---

func TestUpdateHub_Broadcast_NoClients(t *testing.T) {
	hub := api.NewUpdateHub()
	// Should not panic or block with no clients
	hub.Broadcast(api.Message{Type: api.ServiceStatusMsg, Payload: "test"})
}

// --- Ownership checks (#165) ---

// sessionCookieFor creates a DB session for the profile and returns the cookie.
func sessionCookieFor(t *testing.T, profile string) *http.Cookie {
	t.Helper()
	token, err := db.CreateSession(profile, 1)
	if err != nil {
		t.Fatalf("CreateSession(%s): %v", profile, err)
	}
	return &http.Cookie{Name: "hp_session", Value: token}
}

// TestOwnership_DeleteService_Forbidden: non-admin cannot delete a service not visible to them.
func TestOwnership_DeleteService_Forbidden(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("OwnerCat", "blue")
	cats, _ := db.GetCategoriesWithServices("")
	catID := cats[0].ID
	// Service only visible to "andrea", not "markus"
	db.AddService(catID, "AndreaSvc", "http://andrea.svc", "", "", "", false, []string{"andrea"})
	svcCats, _ := db.GetCategoriesWithServices("")
	svcID := svcCats[0].Services[0].ID

	r := chi.NewRouter()
	r.Delete("/manage/service/{id}", srv.HandleDeleteService)

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/manage/service/%d", svcID), nil)
	req.AddCookie(sessionCookieFor(t, "markus")) // markus has no auth entry → non-admin
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("#165: expected 403 Forbidden for non-admin deleting foreign service, got %d", rr.Code)
	}
}

// TestOwnership_DeleteService_Allowed: non-admin can delete a service visible to them.
func TestOwnership_DeleteService_Allowed(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("OwnerCat2", "red")
	cats, _ := db.GetCategoriesWithServices("")
	catID := cats[0].ID
	// Service visible to "markus"
	db.AddService(catID, "MarkusSvc", "http://markus.svc", "", "", "", false, []string{"markus"})
	svcCats, _ := db.GetCategoriesWithServices("")
	svcID := svcCats[0].Services[0].ID

	r := chi.NewRouter()
	r.Delete("/manage/service/{id}", srv.HandleDeleteService)

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/manage/service/%d", svcID), nil)
	req.AddCookie(sessionCookieFor(t, "markus"))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("#165: expected 200 for non-admin deleting own service, got %d", rr.Code)
	}
}

// TestOwnership_UpdateService_Forbidden: non-admin cannot update a service not visible to them.
func TestOwnership_UpdateService_Forbidden(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("OwnerCat3", "green")
	cats, _ := db.GetCategoriesWithServices("")
	catID := cats[0].ID
	db.AddService(catID, "AndreaSvc2", "http://andrea2.svc", "", "", "", false, []string{"andrea"})
	svcCats, _ := db.GetCategoriesWithServices("")
	svcID := svcCats[0].Services[0].ID

	r := chi.NewRouter()
	r.Patch("/manage/service/{id}", srv.HandleUpdateService)

	form := strings.NewReader("name=Updated&url=http://updated.svc")
	req := httptest.NewRequest("PATCH", fmt.Sprintf("/manage/service/%d", svcID), form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sessionCookieFor(t, "markus"))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("#165: expected 403 Forbidden for non-admin updating foreign service, got %d", rr.Code)
	}
}

// TestOwnership_DeleteCategory_Forbidden: non-admin cannot delete a category.
func TestOwnership_DeleteCategory_Forbidden(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("GlobalCat", "purple")
	cats, _ := db.GetCategoriesWithServices("")
	catID := cats[0].ID

	r := chi.NewRouter()
	r.Delete("/manage/category/{id}", srv.HandleDeleteCategory)

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/manage/category/%d", catID), nil)
	req.AddCookie(sessionCookieFor(t, "markus"))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("#165: expected 403 Forbidden for non-admin deleting category, got %d", rr.Code)
	}
}

// TestOwnership_UpdateCategory_Forbidden: non-admin cannot update a category.
func TestOwnership_UpdateCategory_Forbidden(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	db.AddCategory("GlobalCat2", "cyan")
	cats, _ := db.GetCategoriesWithServices("")
	catID := cats[0].ID

	r := chi.NewRouter()
	r.Patch("/manage/category/{id}", srv.HandleUpdateCategory)

	form := strings.NewReader("name=NewName&color=blue")
	req := httptest.NewRequest("PATCH", fmt.Sprintf("/manage/category/%d", catID), form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sessionCookieFor(t, "markus"))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("#165: expected 403 Forbidden for non-admin updating category, got %d", rr.Code)
	}
}

// --- No discovery duplicates in category list (#159) ---

// TestCategoryList_NoDiscoveryDuplicates: category list must not render discovery items inline.
// Discovery items are shown only via the separate discovery-inbox partial.
func TestCategoryList_NoDiscoveryDuplicates(t *testing.T) {
	srv, cleanup := setupTest(t)
	defer cleanup()

	// Seed a discovery item
	db.AddDiscoveryItemExt("ext:test:1", `{"name":"DiscSvc","url":"http://disc.svc"}`, 0)

	db.AddCategory("NormalCat", "blue")
	req := httptest.NewRequest("POST", "/manage/category", strings.NewReader("name=NormalCat&color=blue"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srv.HandleAddCategory(rr, req)

	body := rr.Body.String()
	// The category_list partial must not contain accept-cl or ignore-cl endpoints
	if strings.Contains(body, "accept-cl") || strings.Contains(body, "ignore-cl") {
		t.Errorf("#159 regression: discovery items rendered inline in category list. Body: %s", body[:min(len(body), 500)])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
