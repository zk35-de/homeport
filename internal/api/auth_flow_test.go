package api_test

// Integration tests for the full auth middleware stack (CSRF + RequireAuth).
// Unlike unit tests that call handlers directly, these tests spin up an
// httptest.Server with the real chi router and use a cookie-jar HTTP client,
// mirroring what a browser actually does.
//
// Covered by this file:
//   - Login: correct/wrong credentials, session cookie is set
//   - Logout: POST /logout succeeds without CSRF token (fix for #142)
//   - Session invalidation: protected route redirects to /login after logout
//   - Profile isolation: logged-in user cannot access another user's profile (#146)

import (
	"html/template"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/zk35-de/homeport/internal/api"
	"github.com/zk35-de/homeport/internal/config"
	"github.com/zk35-de/homeport/internal/db"
)

// authRouter builds a minimal chi router with the full auth middleware stack,
// matching the setup in cmd/homeport/main.go.
func authRouter(srv *api.Server, cfg *config.Config) http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(api.CSRFMiddleware)
	r.Use(api.RequireAuth(cfg))
	r.Get("/login", srv.HandleLogin)
	r.Post("/login", srv.HandleLogin)
	r.Post("/logout", srv.HandleLogout)
	r.Get("/", srv.HandleIndex)
	r.Get("/{slug}", srv.HandleIndex)
	return r
}

// manageRouter extends authRouter with /manage routes, mirroring main.go.
func manageRouter(srv *api.Server, cfg *config.Config) http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(api.CSRFMiddleware)
	r.Use(api.RequireAuth(cfg))
	r.Get("/login", srv.HandleLogin)
	r.Post("/login", srv.HandleLogin)
	r.Post("/logout", srv.HandleLogout)
	r.Get("/", srv.HandleIndex)
	r.Route("/manage", func(r chi.Router) {
		r.Get("/", srv.HandleManage)
		r.Post("/category", srv.HandleAddCategory)
		r.Group(func(r chi.Router) {
			r.Use(srv.RequireAdmin)
			r.Get("/auth", srv.HandleManageAuth)
			r.Get("/backup", srv.HandleBackupDownload)
			r.Get("/analytics", srv.HandleAnalytics)
		})
	})
	r.Get("/{slug}", srv.HandleIndex)
	return r
}

// newAuthClient returns an http.Client with a cookie jar that does NOT follow
// redirects, so tests can assert on the redirect response itself.
func newAuthClient(t *testing.T, server *httptest.Server) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // don't follow redirects
		},
	}
}

// setupAuthTest sets up a full auth test environment:
// - temp DB with markus (admin) and andrea profiles, both with passwords
// - stub templates (login + index)
// - httptest.Server with the full middleware stack
func setupAuthTest(t *testing.T) (*httptest.Server, *http.Client, func()) {
	t.Helper()
	srv, cleanup := setupTestWithLogin(t)

	// Force single DB connection so the server goroutine's writes (sessions,
	// passwords) are immediately visible to the test goroutine.
	// Without this, SQLite WAL mode + connection pool causes the test goroutine
	// to read on a different connection that hasn't seen the WAL yet.
	db.DB.SetMaxOpenConns(1)

	// Also need IndexTmpl for HandleIndex – it's already set by setupTest (called inside setupTestWithLogin)

	// Add a minimal login template that renders a form so we can extract the CSRF token
	loginTmpl, err := template.New("login.html").Parse(
		`<form method="post" action="/login">` +
			`<input name="csrf_token" value="{{.CSRFToken}}">` +
			`<input name="profile"><input name="password">` +
			`</form>`,
	)
	if err != nil {
		cleanup()
		t.Fatalf("parse login template: %v", err)
	}
	srv.LoginTmpl = loginTmpl

	// Set passwords: markus=markus (admin), andrea=andrea
	if err := db.SetPassword("markus", "markus"); err != nil {
		cleanup()
		t.Fatalf("SetPassword markus: %v", err)
	}
	if err := db.SetPassword("andrea", "andrea"); err != nil {
		cleanup()
		t.Fatalf("SetPassword andrea: %v", err)
	}
	// Make markus admin
	if err := db.SetAdmin("markus", true); err != nil {
		cleanup()
		t.Fatalf("SetAdmin markus: %v", err)
	}

	cfg := &config.Config{SessionDays: 30}
	srv.Config = cfg // HandleLogin reads s.Config.SessionDays for session TTL
	ts := httptest.NewServer(authRouter(srv, cfg))

	client := newAuthClient(t, ts)

	return ts, client, func() {
		ts.Close()
		cleanup()
	}
}

// doLogin performs a login POST and returns the response.
// It first does a GET /login to obtain the CSRF cookie, then POSTs credentials.
func doLogin(t *testing.T, client *http.Client, base, profile, password string) *http.Response {
	t.Helper()
	// GET /login to seed the CSRF cookie
	resp, err := client.Get(base + "/login")
	if err != nil {
		t.Fatalf("GET /login: %v", err)
	}
	resp.Body.Close()

	// Extract CSRF token from cookie jar
	u, _ := url.Parse(base)
	var csrfToken string
	for _, c := range client.Jar.Cookies(u) {
		if c.Name == "hp_csrf" {
			csrfToken = c.Value
		}
	}

	form := url.Values{
		"profile":    {profile},
		"password":   {password},
		"csrf_token": {csrfToken},
	}
	resp, err = client.PostForm(base+"/login", form)
	if err != nil {
		t.Fatalf("POST /login: %v", err)
	}
	return resp
}

// --- Tests ---

func TestAuthFlow_LoginSuccess(t *testing.T) {
	ts, client, cleanup := setupAuthTest(t)
	defer cleanup()

	resp := doLogin(t, client, ts.URL, "markus", "markus")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected redirect after login, got %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "markus") {
		t.Errorf("expected redirect to /markus, got %q", loc)
	}

	// Session cookie must be set
	u, _ := url.Parse(ts.URL)
	var hasSession bool
	for _, c := range client.Jar.Cookies(u) {
		if c.Name == "hp_session" {
			hasSession = true
		}
	}
	if !hasSession {
		t.Error("expected hp_session cookie after login, none found")
	}
}

func TestAuthFlow_LoginWrongPassword(t *testing.T) {
	ts, client, cleanup := setupAuthTest(t)
	defer cleanup()

	resp := doLogin(t, client, ts.URL, "markus", "wrongpassword")
	defer resp.Body.Close()

	// Login page re-rendered (200), no session cookie
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 on failed login, got %d", resp.StatusCode)
	}
	u, _ := url.Parse(ts.URL)
	for _, c := range client.Jar.Cookies(u) {
		if c.Name == "hp_session" {
			t.Error("session cookie must not be set after failed login")
		}
	}
}

// TestAuthFlow_LogoutNoCsrf verifies that POST /logout succeeds without a CSRF
// token (fix for #142). With auth active, /logout must not return 403.
func TestAuthFlow_LogoutNoCsrf(t *testing.T) {
	ts, client, cleanup := setupAuthTest(t)
	defer cleanup()

	// Login first
	loginResp := doLogin(t, client, ts.URL, "markus", "markus")
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login failed, got %d – test precondition not met", loginResp.StatusCode)
	}

	// POST /logout WITHOUT a CSRF token – must not return 403
	resp, err := client.Post(ts.URL+"/logout", "application/x-www-form-urlencoded", nil)
	if err != nil {
		t.Fatalf("POST /logout: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		t.Error("POST /logout returned 403 CSRF error – logout is not exempt (#142)")
	}
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected redirect after logout, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Errorf("expected redirect to /login after logout, got %q", loc)
	}
}

// TestAuthFlow_SessionInvalidatedAfterLogout verifies that a protected route
// redirects to /login after the session has been logged out.
func TestAuthFlow_SessionInvalidatedAfterLogout(t *testing.T) {
	ts, client, cleanup := setupAuthTest(t)
	defer cleanup()

	// Login
	loginResp := doLogin(t, client, ts.URL, "markus", "markus")
	loginResp.Body.Close()

	// Verify access works while logged in
	resp, _ := client.Get(ts.URL + "/markus")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 while logged in, got %d", resp.StatusCode)
	}

	// Logout
	logoutResp, _ := client.Post(ts.URL+"/logout", "", nil)
	logoutResp.Body.Close()

	// Access after logout → must redirect to /login
	resp, _ = client.Get(ts.URL + "/markus")
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected redirect to /login after logout, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Errorf("expected Location: /login, got %q", loc)
	}
}

// setupAuthTestMixed creates a setup where markus has a password (admin) but
// andrea has none. Used to test per-profile auth (#145).
func setupAuthTestMixed(t *testing.T) (*httptest.Server, *http.Client, func()) {
	t.Helper()
	srv, cleanup := setupTestWithLogin(t)
	db.DB.SetMaxOpenConns(1)

	loginTmpl, err := template.New("login.html").Parse(
		`<form method="post" action="/login">` +
			`<input name="csrf_token" value="{{.CSRFToken}}">` +
			`<input name="profile"><input name="password">` +
			`</form>`,
	)
	if err != nil {
		cleanup()
		t.Fatalf("parse login template: %v", err)
	}
	srv.LoginTmpl = loginTmpl

	// Only markus gets a password; andrea has none.
	if err := db.SetPassword("markus", "markus"); err != nil {
		cleanup()
		t.Fatalf("SetPassword markus: %v", err)
	}
	if err := db.SetAdmin("markus", true); err != nil {
		cleanup()
		t.Fatalf("SetAdmin markus: %v", err)
	}

	cfg := &config.Config{SessionDays: 30}
	srv.Config = cfg
	ts := httptest.NewServer(authRouter(srv, cfg))
	client := newAuthClient(t, ts)

	return ts, client, func() { ts.Close(); cleanup() }
}

// TestAuthFlow_PasswordlessProfileAccessible verifies that a profile without a
// password is directly accessible even when another profile has a password (#145).
func TestAuthFlow_PasswordlessProfileAccessible(t *testing.T) {
	ts, client, cleanup := setupAuthTestMixed(t)
	defer cleanup()

	// andrea has no password → must be accessible without login
	resp, err := client.Get(ts.URL + "/andrea")
	if err != nil {
		t.Fatalf("GET /andrea: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/andrea (no password): expected 200, got %d", resp.StatusCode)
	}
}

// TestAuthFlow_PasswordedProfileRequiresAuth verifies that a profile WITH a
// password still requires authentication (#145).
func TestAuthFlow_PasswordedProfileRequiresAuth(t *testing.T) {
	ts, client, cleanup := setupAuthTestMixed(t)
	defer cleanup()

	// markus has a password → must redirect to /login without a session
	resp, err := client.Get(ts.URL + "/markus")
	if err != nil {
		t.Fatalf("GET /markus: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("/markus (has password): expected 303 redirect to /login, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}

// setupAuthTestNonAdmin creates a setup where both markus and andrea have
// passwords but markus is explicitly NOT admin. Used to test profile isolation
// without the admin bypass (#146).
func setupAuthTestNonAdmin(t *testing.T) (*httptest.Server, *http.Client, func()) {
	t.Helper()
	srv, cleanup := setupTestWithLogin(t)
	db.DB.SetMaxOpenConns(1)

	loginTmpl, err := template.New("login.html").Parse(
		`<form method="post" action="/login">` +
			`<input name="csrf_token" value="{{.CSRFToken}}">` +
			`<input name="profile"><input name="password">` +
			`</form>`,
	)
	if err != nil {
		cleanup()
		t.Fatalf("parse login template: %v", err)
	}
	srv.LoginTmpl = loginTmpl

	// First SetPassword call auto-grants admin; we immediately revoke it.
	if err := db.SetPassword("markus", "markus"); err != nil {
		cleanup()
		t.Fatalf("SetPassword markus: %v", err)
	}
	if err := db.SetAdmin("markus", false); err != nil {
		cleanup()
		t.Fatalf("SetAdmin markus false: %v", err)
	}
	if err := db.SetPassword("andrea", "andrea"); err != nil {
		cleanup()
		t.Fatalf("SetPassword andrea: %v", err)
	}

	cfg := &config.Config{SessionDays: 30}
	srv.Config = cfg
	ts := httptest.NewServer(authRouter(srv, cfg))
	client := newAuthClient(t, ts)

	return ts, client, func() { ts.Close(); cleanup() }
}

// TestAuthFlow_ProfileIsolation verifies that a non-admin user logged in as
// markus cannot access andrea's profile page (#146).
func TestAuthFlow_ProfileIsolation(t *testing.T) {
	ts, client, cleanup := setupAuthTestNonAdmin(t)
	defer cleanup()

	loginResp := doLogin(t, client, ts.URL, "markus", "markus")
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login failed, got %d", loginResp.StatusCode)
	}

	// markus can access his own profile
	resp, _ := client.Get(ts.URL + "/markus")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/markus: expected 200, got %d", resp.StatusCode)
	}

	// markus must NOT access andrea's profile
	resp, _ = client.Get(ts.URL + "/andrea")
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("/andrea: expected 403, got %d", resp.StatusCode)
	}
}

// TestAuthFlow_AdminCanAccessAllProfiles verifies that an admin user can access
// any profile page, including other users' profiles (#146).
func TestAuthFlow_AdminCanAccessAllProfiles(t *testing.T) {
	ts, client, cleanup := setupAuthTest(t) // markus = admin
	defer cleanup()

	loginResp := doLogin(t, client, ts.URL, "markus", "markus")
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login failed, got %d", loginResp.StatusCode)
	}

	// Admin markus can access andrea's profile
	resp, _ := client.Get(ts.URL + "/andrea")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/andrea (admin session): expected 200, got %d", resp.StatusCode)
	}
}

// TestAuthFlow_LoggedInUserCanAccessPasswordlessProfile verifies that a logged-in
// user (as any profile) can still access a passwordless profile (#145 + #146
// interaction: passwordless bypass must survive isolation logic).
func TestAuthFlow_LoggedInUserCanAccessPasswordlessProfile(t *testing.T) {
	ts, client, cleanup := setupAuthTestMixed(t) // markus=admin+password, andrea=no password
	defer cleanup()

	loginResp := doLogin(t, client, ts.URL, "markus", "markus")
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login failed, got %d", loginResp.StatusCode)
	}

	// andrea has no password → isolation does not apply, always accessible
	resp, _ := client.Get(ts.URL + "/andrea")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/andrea (passwordless, logged in as markus): expected 200, got %d", resp.StatusCode)
	}
}

// --- /manage access control tests (#143) ---

// setupManageTest sets up a full test env with manage routes registered,
// using setupAuthTestNonAdmin (markus=non-admin, andrea=non-admin).
func setupManageTest(t *testing.T) (*httptest.Server, *http.Client, func()) {
	t.Helper()
	srv, cleanup := setupTestWithLogin(t)
	db.DB.SetMaxOpenConns(1)

	loginTmpl, err := template.New("login.html").Parse(
		`<form method="post" action="/login">` +
			`<input name="csrf_token" value="{{.CSRFToken}}">` +
			`<input name="profile"><input name="password">` +
			`</form>`,
	)
	if err != nil {
		cleanup()
		t.Fatalf("parse login template: %v", err)
	}
	srv.LoginTmpl = loginTmpl

	if err := db.SetPassword("markus", "markus"); err != nil {
		cleanup()
		t.Fatalf("SetPassword markus: %v", err)
	}
	if err := db.SetAdmin("markus", false); err != nil {
		cleanup()
		t.Fatalf("SetAdmin markus false: %v", err)
	}
	if err := db.SetPassword("andrea", "andrea"); err != nil {
		cleanup()
		t.Fatalf("SetPassword andrea: %v", err)
	}

	cfg := &config.Config{SessionDays: 30}
	srv.Config = cfg
	ts := httptest.NewServer(manageRouter(srv, cfg))
	client := newAuthClient(t, ts)

	return ts, client, func() { ts.Close(); cleanup() }
}

// setupManageTestAdmin is like setupManageTest but markus is admin.
func setupManageTestAdmin(t *testing.T) (*httptest.Server, *http.Client, func()) {
	t.Helper()
	srv, cleanup := setupTestWithLogin(t)
	db.DB.SetMaxOpenConns(1)

	loginTmpl, err := template.New("login.html").Parse(
		`<form method="post" action="/login">` +
			`<input name="csrf_token" value="{{.CSRFToken}}">` +
			`<input name="profile"><input name="password">` +
			`</form>`,
	)
	if err != nil {
		cleanup()
		t.Fatalf("parse login template: %v", err)
	}
	srv.LoginTmpl = loginTmpl

	if err := db.SetPassword("markus", "markus"); err != nil {
		cleanup()
		t.Fatalf("SetPassword markus: %v", err)
	}
	if err := db.SetAdmin("markus", true); err != nil {
		cleanup()
		t.Fatalf("SetAdmin markus: %v", err)
	}

	cfg := &config.Config{SessionDays: 30}
	srv.Config = cfg
	ts := httptest.NewServer(manageRouter(srv, cfg))
	client := newAuthClient(t, ts)

	return ts, client, func() { ts.Close(); cleanup() }
}

// TestManageAccess_NonAdminCanGetManage verifies that a non-admin authenticated
// user can access GET /manage (#143).
func TestManageAccess_NonAdminCanGetManage(t *testing.T) {
	ts, client, cleanup := setupManageTest(t)
	defer cleanup()

	loginResp := doLogin(t, client, ts.URL, "markus", "markus")
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login failed, got %d", loginResp.StatusCode)
	}

	resp, err := client.Get(ts.URL + "/manage")
	if err != nil {
		t.Fatalf("GET /manage: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/manage (non-admin): expected 200, got %d", resp.StatusCode)
	}
}

// TestManageAccess_NonAdminBlockedFromAdminRoutes verifies that a non-admin
// user cannot access admin-only manage routes (#143).
func TestManageAccess_NonAdminBlockedFromAdminRoutes(t *testing.T) {
	ts, client, cleanup := setupManageTest(t)
	defer cleanup()

	loginResp := doLogin(t, client, ts.URL, "markus", "markus")
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login failed, got %d", loginResp.StatusCode)
	}

	adminRoutes := []string{"/manage/auth", "/manage/backup", "/manage/analytics"}
	for _, route := range adminRoutes {
		resp, err := client.Get(ts.URL + route)
		if err != nil {
			t.Fatalf("GET %s: %v", route, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s (non-admin): expected 403, got %d", route, resp.StatusCode)
		}
	}
}

// TestManageAccess_AdminCanAccessAll verifies that an admin user is NOT blocked
// by the admin middleware on any /manage route (#143). Template errors (500)
// are expected in the stub-template test environment and do not indicate an
// access-control failure.
func TestManageAccess_AdminCanAccessAll(t *testing.T) {
	ts, client, cleanup := setupManageTestAdmin(t)
	defer cleanup()

	loginResp := doLogin(t, client, ts.URL, "markus", "markus")
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login failed, got %d", loginResp.StatusCode)
	}

	// Test that admin is not blocked (403) on any manage route.
	// 200 = handler ran fine; 500 = handler ran but template missing in test env.
	adminRoutes := []string{"/manage", "/manage/auth", "/manage/backup", "/manage/analytics"}
	for _, route := range adminRoutes {
		resp, err := client.Get(ts.URL + route)
		if err != nil {
			t.Fatalf("GET %s: %v", route, err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusForbidden {
			t.Errorf("%s (admin): got 403 Forbidden – admin should not be blocked", route)
		}
	}
}

// TestManageAccess_UnauthenticatedRedirected verifies that unauthenticated
// requests to /manage are redirected to /login (not 403).
func TestManageAccess_UnauthenticatedRedirected(t *testing.T) {
	ts, _, cleanup := setupManageTest(t)
	defer cleanup()

	// Fresh client without any session
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(ts.URL + "/manage")
	if err != nil {
		t.Fatalf("GET /manage: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("/manage (no session): expected 303 redirect to /login, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}
