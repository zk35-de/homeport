package api

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/zk35-de/homeport/internal/config"
	"github.com/zk35-de/homeport/internal/db"
	"github.com/zk35-de/homeport/internal/i18n"
)

const sessionCookie = "hp_session"

// SessionProfile extracts the authenticated profile from the request cookie.
// Returns "" if not authenticated or auth is disabled.
func SessionProfile(r *http.Request) string {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	return db.GetSession(c.Value)
}

// authActive returns true if auth should be enforced: either explicitly enabled
// via HOMEPORT_AUTH=true, or implicitly because at least one password is set.
func authActive(cfg *config.Config) bool {
	return (cfg != nil && cfg.AuthEnabled) || db.HasAnyPassword()
}

// RequireAdmin middleware: if auth is enabled, only admin profiles may proceed.
// Must run after RequireAuth (assumes valid session already verified).
// Exception: when auth is enabled but no password exists yet (setup mode),
// only /manage/auth routes are accessible to allow initial password setup.
func (s *Server) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authActive(s.Config) {
			// Setup mode: no passwords set yet → only allow auth management routes
			if !db.HasAnyPassword() {
				path := r.URL.Path
				if path == "/manage/auth" || path == "/manage/auth/password" {
					next.ServeHTTP(w, r)
					return
				}
				http.Redirect(w, r, "/manage/auth", http.StatusSeeOther)
				return
			}
			profile := SessionProfile(r)
			a, err := db.GetUserAuth(profile)
			if err != nil || a == nil || !a.IsAdmin {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// resolveProfileSlug returns the profile slug for a profile page path.
// Returns "" for non-profile paths (/manage, /api/..., unknown slugs).
// Safe to call with db.DB == nil (returns "").
func resolveProfileSlug(path string) string {
	if db.DB == nil {
		return ""
	}
	if path == "/" {
		p, err := db.GetDefaultProfile()
		if err != nil || p == nil {
			return ""
		}
		return p.Slug
	}
	// Only single-segment paths can be profile slugs (/markus, /andrea).
	// Multi-segment paths (/manage/auth, /api/...) are never profile pages.
	trimmed := strings.TrimPrefix(path, "/")
	if trimmed == "" || strings.ContainsRune(trimmed, '/') {
		return ""
	}
	p, err := db.GetProfileBySlug(trimmed)
	if err != nil || p == nil {
		return ""
	}
	return p.Slug
}

// profilePathNeedsAuth returns true if the given path requires authentication.
// Returns false only for profile pages (/ or /{slug}) where the profile has
// no password set — those are always accessible without a session.
func profilePathNeedsAuth(path string) bool {
	slug := resolveProfileSlug(path)
	if slug == "" {
		return true // not a profile page → auth required
	}
	auth, err := db.GetUserAuth(slug)
	if err != nil {
		return true
	}
	return auth != nil // has password → needs auth; no password → open
}

// RequireAuth middleware: if auth is enabled, check session.
// Unauthenticated requests are redirected to /login (or shown public profile).
// Active sessions are renewed (rolling/sliding expiry) when less than
// SessionDays/2 remain, so inactive sessions expire after SessionDays.
func RequireAuth(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !authActive(cfg) {
				next.ServeHTTP(w, r)
				return
			}

			// Whitelist: always accessible without auth
			path := r.URL.Path
			if path == "/login" || path == "/logout" ||
				strings.HasPrefix(path, "/static/") ||
				path == "/api/health" {
				next.ServeHTTP(w, r)
				return
			}

			// Setup mode: auth enabled but no password set yet → allow /manage/auth
			if !db.HasAnyPassword() &&
				(path == "/manage/auth" || path == "/manage/auth/password") {
				next.ServeHTTP(w, r)
				return
			}

			// Rolling session: check expiry and renew if within threshold
			var profile string
			if c, err := r.Cookie(sessionCookie); err == nil {
				if info := db.GetSessionInfo(c.Value); info != nil {
					profile = info.Profile
					threshold := time.Duration(cfg.SessionDays/2) * 24 * time.Hour
					if time.Until(info.ExpiresAt) < threshold {
						if extErr := db.ExtendSession(c.Value, cfg.SessionDays); extErr == nil {
							http.SetCookie(w, &http.Cookie{
								Name:     sessionCookie,
								Value:    c.Value,
								Path:     "/",
								MaxAge:   cfg.SessionDays * 86400,
								HttpOnly: true,
								SameSite: http.SameSiteLaxMode,
							})
						}
					}
				}
			}
			if profile != "" {
				// Profile isolation: non-admin users may only access their own
				// profile page. Passwordless profiles remain open to everyone.
				// Admins bypass isolation and can access all profiles.
				if targetSlug := resolveProfileSlug(path); targetSlug != "" {
					targetAuth, _ := db.GetUserAuth(targetSlug)
					if targetAuth != nil { // target profile is password-protected
						userAuth, _ := db.GetUserAuth(profile)
						isAdmin := userAuth != nil && userAuth.IsAdmin
						if !isAdmin && profile != targetSlug {
							http.Error(w, "Forbidden", http.StatusForbidden)
							return
						}
					}
				}
				next.ServeHTTP(w, r)
				return
			}

			// Unauthenticated: allow passwordless profile pages
			if !profilePathNeedsAuth(path) {
				next.ServeHTTP(w, r)
				return
			}

			// Legacy: explicit public profile config override
			if cfg != nil && cfg.PublicProfile != "" &&
				(path == "/" || path == "/"+cfg.PublicProfile) {
				next.ServeHTTP(w, r)
				return
			}

			http.Redirect(w, r, "/login", http.StatusSeeOther)
		})
	}
}

// HandleLogin GET/POST /login
func (s *Server) HandleLogin(w http.ResponseWriter, r *http.Request) {
	t := i18n.T("de")
	if r.Method == http.MethodGet {
		s.renderLogin(w, "", "de")
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	profile := r.FormValue("profile")
	password := r.FormValue("password")

	if !LoginRateLimit(r) {
		time.Sleep(loginDelay)
		s.renderLogin(w, t("login.error.ratelimit"), "de")
		return
	}

	if !db.CheckPassword(profile, password) {
		s.renderLogin(w, t("login.error.invalid"), "de")
		return
	}
	LoginReset(r)

	sessionDays := 30
	if s.Config != nil {
		sessionDays = s.Config.SessionDays
	}
	token, err := db.CreateSession(profile, sessionDays)
	if err != nil {
		slog.Error("create session", "profile", profile, "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   sessionDays * 86400,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	// Redirect to profile page
	http.Redirect(w, r, "/"+profile, http.StatusSeeOther)
}

// HandleLogout POST /logout
func (s *Server) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		db.DeleteSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) renderLogin(w http.ResponseWriter, errMsg string, lang string) {
	data := struct {
		i18n.Translator
		Error    string
		Profiles []db.Profile
	}{
		Translator: i18n.NewTranslator(lang),
		Error:      errMsg,
	}
	profiles, err := db.GetProfiles()
	if err != nil {
		slog.Error("GetProfiles", "err", err)
	}
	// Only show profiles that have a password set
	for _, p := range profiles {
		a, authErr := db.GetUserAuth(p.Slug)
		if authErr != nil {
			slog.Error("GetUserAuth", "profile", p.Slug, "err", authErr)
			continue
		}
		if a != nil {
			data.Profiles = append(data.Profiles, p)
		}
	}
	if err := s.LoginTmpl.ExecuteTemplate(w, "login.html", data); err != nil {
		slog.Error("login template", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// HandleManageAuth GET /manage/auth – Passwörter verwalten (Admin only via RequireAdmin middleware)
func (s *Server) HandleManageAuth(w http.ResponseWriter, r *http.Request) {
	profiles, err := db.GetProfiles()
	if err != nil {
		slog.Error("GetProfiles", "err", err)
	}
	auths, err := db.GetAllUserAuth()
	if err != nil {
		slog.Error("GetAllUserAuth", "err", err)
	}
	authMap := make(map[string]db.UserAuth)
	for _, a := range auths {
		authMap[a.Profile] = a
	}

	lang := "de"
	if def, err := db.GetDefaultProfile(); err == nil && def != nil {
		if prefs, err := db.GetUserPreferences(def.Slug); err == nil && prefs != nil && prefs.Language != "" {
			lang = prefs.Language
		}
	}

	data := struct {
		i18n.Translator
		Profiles []db.Profile
		AuthMap  map[string]db.UserAuth
	}{Translator: i18n.NewTranslator(lang), Profiles: profiles, AuthMap: authMap}

	if err := s.ManageTmpl.ExecuteTemplate(w, "auth_list", data); err != nil {
		slog.Error("auth_list template", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// HandleSetPassword POST /manage/auth/password
func (s *Server) HandleSetPassword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	target := r.FormValue("profile")
	password := r.FormValue("password")
	if target == "" || password == "" {
		http.Error(w, "profile and password required", http.StatusBadRequest)
		return
	}
	if err := db.SetPassword(target, password); err != nil {
		slog.Error("SetPassword", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	s.HandleManageAuth(w, r)
}

// HandleDeletePassword DELETE /manage/auth/password/{profile}
func (s *Server) HandleDeletePassword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	target := r.FormValue("profile")
	if err := db.DeleteUserAuth(target); err != nil {
		slog.Error("DeleteUserAuth", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	s.HandleManageAuth(w, r)
}
