package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"
)

// --- Security Headers Middleware ---

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self'; "+
				"style-src 'self'; "+
				"img-src 'self' data:; "+
				"connect-src 'self'; "+
				"font-src 'self'; "+
				"frame-ancestors 'self'")
		next.ServeHTTP(w, r)
	})
}

// --- CSRF (Double-Submit Cookie) ---

const csrfCookie = "hp_csrf"
const csrfHeader = "X-CSRF-Token"
const csrfFormField = "csrf_token"

// CSRFMiddleware validates CSRF tokens for all state-changing requests.
// GET/HEAD/OPTIONS are exempt. HTMX sends the token via X-CSRF-Token header.
// Regular forms send it as a hidden field.
func CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Safe methods exempt
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			ensureCSRFCookie(w, r)
			next.ServeHTTP(w, r)
			return
		}

		// Bearer token auth was removed in #97 – the blanket /api/ exemption
		// no longer has a security justification. Only read-only /api/ GETs
		// are exempt (already covered by the safe-methods check above).
		// PATCH /api/user/preferences and other state-changing API calls
		// must pass CSRF validation like any other request.

		// Login POST is exempt (no session yet, can't have valid CSRF)
		if r.URL.Path == "/login" {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie(csrfCookie)
		if err != nil || cookie.Value == "" {
			http.Error(w, "CSRF token missing", http.StatusForbidden)
			return
		}

		// HTMX sends via header, plain forms via hidden field
		token := r.Header.Get(csrfHeader)
		if token == "" {
			token = r.FormValue(csrfFormField)
		}

		if token == "" || token != cookie.Value {
			http.Error(w, "CSRF validation failed", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func ensureCSRFCookie(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie(csrfCookie); err == nil && c.Value != "" {
		return c.Value
	}
	b := make([]byte, 16)
	rand.Read(b)
	token := hex.EncodeToString(b)
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookie,
		Value:    token,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		HttpOnly: false, // must be readable by JS for HTMX header injection
	})
	return token
}

// CSRFToken returns the current CSRF token for the request (for template use).
func CSRFToken(r *http.Request) string {
	if c, err := r.Cookie(csrfCookie); err == nil {
		return c.Value
	}
	return ""
}

// --- Rate Limiting for /login ---

type loginAttempt struct {
	count    int
	lastSeen time.Time
}

var (
	loginMu       sync.Mutex
	loginAttempts = make(map[string]*loginAttempt)
)

const loginMaxAttempts = 5
const loginWindow = 5 * time.Minute
const loginDelay = 2 * time.Second

// LoginRateLimit checks and records login attempts by IP.
// Returns false if the IP is rate-limited.
func LoginRateLimit(r *http.Request) bool {
	ip := realIP(r)

	loginMu.Lock()
	defer loginMu.Unlock()

	attempt, ok := loginAttempts[ip]
	if !ok {
		loginAttempts[ip] = &loginAttempt{count: 1, lastSeen: time.Now()}
		return true
	}

	// Reset window if enough time passed
	if time.Since(attempt.lastSeen) > loginWindow {
		attempt.count = 1
		attempt.lastSeen = time.Now()
		return true
	}

	attempt.count++
	attempt.lastSeen = time.Now()
	return attempt.count <= loginMaxAttempts
}

// LoginReset clears the attempt counter for an IP on successful login.
func LoginReset(r *http.Request) {
	ip := realIP(r)
	loginMu.Lock()
	delete(loginAttempts, ip)
	loginMu.Unlock()
}

func realIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx > 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Strip port
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx > 0 {
		return addr[:idx]
	}
	return addr
}

// Periodic cleanup to avoid unbounded map growth
func init() {
	go func() {
		for range time.Tick(10 * time.Minute) {
			loginMu.Lock()
			for ip, a := range loginAttempts {
				if time.Since(a.lastSeen) > loginWindow*2 {
					delete(loginAttempts, ip)
				}
			}
			loginMu.Unlock()
		}
	}()
}
