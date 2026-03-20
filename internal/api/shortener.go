package api

import (
	"crypto/rand"
	"encoding/json"
	"math/big"
	"net/http"

	"github.com/go-chi/chi/v5"
	"git.zk35.de/secalpha/homeport/internal/db"
)

const shortCodeAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

func generateCode() (string, error) {
	code := make([]byte, 6)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(shortCodeAlphabet))))
		if err != nil {
			return "", err
		}
		code[i] = shortCodeAlphabet[n.Int64()]
	}
	return string(code), nil
}

// RegisterShortenerPublicRoutes registers the public redirect route (no auth).
func RegisterShortenerPublicRoutes(r chi.Router) {
	r.Get("/s/{code}", handleRedirect)
}

// RegisterShortenerAPIRoutes registers the auth-protected API routes.
func RegisterShortenerAPIRoutes(r chi.Router) {
	r.Post("/shorten", handleShorten)
	r.Get("/links", handleGetLinks)
	r.Delete("/links/{code}", handleDeleteLink)
}

func handleShorten(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL  string `json:"url"`
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.URL == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	code := body.Code
	if code == "" {
		var err error
		for i := 0; i < 5; i++ {
			code, err = generateCode()
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			existing, _ := db.GetShortURL(code)
			if existing == nil {
				break
			}
			code = ""
		}
		if code == "" {
			http.Error(w, "Could not generate unique code", http.StatusInternalServerError)
			return
		}
	} else {
		existing, _ := db.GetShortURL(code)
		if existing != nil {
			http.Error(w, "Code already in use", http.StatusConflict)
			return
		}
	}

	if err := db.CreateShortURL(code, body.URL); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"code":      code,
		"short_url": "/s/" + code,
	})
}

func handleGetLinks(w http.ResponseWriter, r *http.Request) {
	urls, err := db.GetAllShortURLs()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if urls == nil {
		urls = []db.ShortURL{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(urls)
}

func handleDeleteLink(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if err := db.DeleteShortURL(code); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func handleRedirect(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	entry, err := db.GetShortURL(code)
	if err != nil || entry == nil {
		http.NotFound(w, r)
		return
	}
	_ = db.IncrementClicks(code)
	http.Redirect(w, r, entry.URL, http.StatusFound)
}
