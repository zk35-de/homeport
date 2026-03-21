package api

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var faviconClient = &http.Client{Timeout: 5 * time.Second}

// HandleFavicon proxies a favicon for a given URL.
// GET /api/favicon?url=https://example.com
// The server fetches the favicon so internal services work without CORS issues.
func HandleFavicon(w http.ResponseWriter, r *http.Request) {
	rawURL := r.URL.Query().Get("url")
	if rawURL == "" {
		http.Error(w, "missing url", http.StatusBadRequest)
		return
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		http.Error(w, "invalid url", http.StatusBadRequest)
		return
	}

	base := parsed.Scheme + "://" + parsed.Host

	// Try /favicon.ico first
	if img, ct := fetchImage(base + "/favicon.ico"); img != nil {
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Write(img)
		return
	}

	// Try DuckDuckGo as fallback (for public domains)
	ddgURL := "https://icons.duckduckgo.com/ip3/" + parsed.Hostname() + ".ico"
	if img, ct := fetchImage(ddgURL); img != nil {
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Write(img)
		return
	}

	http.NotFound(w, r)
}

func fetchImage(url string) ([]byte, string) {
	resp, err := faviconClient.Get(url)
	if err != nil {
		return nil, ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, ""
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/") && ct != "image/x-icon" && !strings.Contains(ct, "icon") {
		// Tolerate common favicon content-types
		if ct != "application/octet-stream" && !strings.HasPrefix(ct, "image") {
			return nil, ""
		}
	}

	// Limit to 100KB
	data, err := io.ReadAll(io.LimitReader(resp.Body, 100*1024))
	if err != nil || len(data) == 0 {
		return nil, ""
	}

	if ct == "" {
		ct = "image/x-icon"
	}
	return data, ct
}
