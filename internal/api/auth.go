package api

import (
	"html/template"
	"log"
	"net/http"
	"strings"

	"git.zk35.de/secalpha/homeport/internal/config"
	"git.zk35.de/secalpha/homeport/internal/db"
)

const sessionCookie = "hp_session"

var LoginTmpl *template.Template

// SessionProfile extracts the authenticated profile from the request cookie.
// Returns "" if not authenticated or auth is disabled.
func SessionProfile(r *http.Request) string {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	return db.GetSession(c.Value)
}

// RequireAuth middleware: if auth is enabled, check session.
// Unauthenticated requests are redirected to /login (or shown public profile).
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
				strings.HasPrefix(path, "/s/") ||
				path == "/api/health" {
				next.ServeHTTP(w, r)
				return
			}

			profile := SessionProfile(r)
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
func HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		renderLogin(w, "")
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	profile := r.FormValue("profile")
	password := r.FormValue("password")

	if !db.CheckPassword(profile, password) {
		renderLogin(w, "Falsches Profil oder Passwort.")
		return
	}

	sessionDays := 30
	if appConfig != nil {
		sessionDays = appConfig.SessionDays
	}
	token, err := db.CreateSession(profile, sessionDays)
	if err != nil {
		log.Printf("Login: failed to create session for %s: %v", profile, err)
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
func HandleLogout(w http.ResponseWriter, r *http.Request) {
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

func renderLogin(w http.ResponseWriter, errMsg string) {
	data := struct {
		Error    string
		Profiles []db.Profile
	}{
		Error: errMsg,
	}
	profiles, _ := db.GetProfiles()
	// Only show profiles that have a password set
	for _, p := range profiles {
		if a, _ := db.GetUserAuth(p.Slug); a != nil {
			data.Profiles = append(data.Profiles, p)
		}
	}
	if err := LoginTmpl.ExecuteTemplate(w, "login.html", data); err != nil {
		log.Printf("Login template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// HandleManageAuth GET /manage/auth – Passwörter verwalten (Admin only)
func HandleManageAuth(w http.ResponseWriter, r *http.Request) {
	// In auth mode: only admin can access this
	if appConfig != nil && appConfig.AuthEnabled {
		profile := SessionProfile(r)
		if a, _ := db.GetUserAuth(profile); a == nil || !a.IsAdmin {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}

	profiles, _ := db.GetProfiles()
	auths, _ := db.GetAllUserAuth()
	authMap := make(map[string]db.UserAuth)
	for _, a := range auths {
		authMap[a.Profile] = a
	}

	data := struct {
		Profiles []db.Profile
		AuthMap  map[string]db.UserAuth
	}{Profiles: profiles, AuthMap: authMap}

	if err := ManageTmpl.ExecuteTemplate(w, "auth_list", data); err != nil {
		log.Printf("auth_list template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// HandleSetPassword POST /manage/auth/password
func HandleSetPassword(w http.ResponseWriter, r *http.Request) {
	if appConfig != nil && appConfig.AuthEnabled {
		profile := SessionProfile(r)
		if a, _ := db.GetUserAuth(profile); a == nil || !a.IsAdmin {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}
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
		log.Printf("SetPassword error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	HandleManageAuth(w, r)
}

// HandleDeletePassword DELETE /manage/auth/password/{profile}
func HandleDeletePassword(w http.ResponseWriter, r *http.Request) {
	if appConfig != nil && appConfig.AuthEnabled {
		profile := SessionProfile(r)
		if a, _ := db.GetUserAuth(profile); a == nil || !a.IsAdmin {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	target := r.FormValue("profile")
	if err := db.DeleteUserAuth(target); err != nil {
		log.Printf("DeleteUserAuth error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	HandleManageAuth(w, r)
}
