package api

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"git.zk35.de/secalpha/homeport/internal/config"
	"git.zk35.de/secalpha/homeport/internal/db"
	"git.zk35.de/secalpha/homeport/internal/i18n"
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

// RequireAdmin middleware: if auth is enabled, only admin profiles may proceed.
// Must run after RequireAuth (assumes valid session already verified).
func (s *Server) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.Config != nil && s.Config.AuthEnabled {
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

// RequireAuth middleware: if auth is enabled, check session.
// Unauthenticated requests are redirected to /login (or shown public profile).
// Active sessions are renewed (rolling/sliding expiry) when less than
// SessionDays/2 remain, so inactive sessions expire after SessionDays.
func RequireAuth(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.AuthEnabled {
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
				next.ServeHTTP(w, r)
				return
			}

			// Unauthenticated: serve public profile or redirect to login
			if cfg.PublicProfile != "" &&
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
