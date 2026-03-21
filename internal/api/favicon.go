package api

import (
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var faviconClient = &http.Client{
	Timeout: 5 * time.Second,
	// Follow redirects (default behaviour)
}

// iconLinkRe matches <link rel="icon|shortcut icon" href="...">
var iconLinkRe = regexp.MustCompile(`(?i)<link[^>]+rel=["'](?:shortcut )?icon["'][^>]+href=["']([^"']+)["']|<link[^>]+href=["']([^"']+)["'][^>]+rel=["'](?:shortcut )?icon["']`)

// HandleFavicon proxies a favicon for a given URL.
// GET /api/favicon?url=https://example.com
// Strategy:
//  1. Try {base}/favicon.ico
//  2. Fetch HTML, parse <link rel="icon"> → try that path
//  3. Fallback: DuckDuckGo icons (public domains only)
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

	// 1. /favicon.ico
	if img, ct := fetchImage(base + "/favicon.ico"); img != nil {
		serveImage(w, img, ct)
		return
	}

	// 2. Parse HTML for <link rel="icon">
	if iconPath := fetchIconPathFromHTML(base+"/", parsed); iconPath != "" {
		if img, ct := fetchImage(iconPath); img != nil {
			serveImage(w, img, ct)
			return
		}
	}

	// 3. DuckDuckGo fallback (works for public domains)
	ddgURL := "https://icons.duckduckgo.com/ip3/" + parsed.Hostname() + ".ico"
	if img, ct := fetchImage(ddgURL); img != nil {
		serveImage(w, img, ct)
		return
	}

	http.NotFound(w, r)
}

func serveImage(w http.ResponseWriter, data []byte, ct string) {
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(data)
}

// fetchIconPathFromHTML fetches the page HTML and extracts the first icon href,
// resolving it to an absolute URL.
func fetchIconPathFromHTML(pageURL string, base *url.URL) string {
	resp, err := faviconClient.Get(pageURL)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	// Read at most 32KB – enough to find <head> content
	buf, err := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if err != nil {
		return ""
	}

	matches := iconLinkRe.FindSubmatch(buf)
	if matches == nil {
		return ""
	}

	// Group 1 or 2 depending on attribute order
	href := string(matches[1])
	if href == "" {
		href = string(matches[2])
	}
	if href == "" {
		return ""
	}

	// Resolve relative to base
	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}
	return base.ResolveReference(ref).String()
}


func fetchImage(targetURL string) ([]byte, string) {
	resp, err := faviconClient.Get(targetURL)
	if err != nil {
		return nil, ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, ""
	}

	ct := resp.Header.Get("Content-Type")
	// Accept image/* and common icon content types; also octet-stream (some servers)
	if ct != "" &&
		!strings.HasPrefix(ct, "image/") &&
		!strings.Contains(ct, "icon") &&
		ct != "application/octet-stream" {
		return nil, ""
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 100*1024))
	if err != nil || len(data) == 0 {
		return nil, ""
	}

	if ct == "" || ct == "application/octet-stream" {
		ct = "image/x-icon"
	}
	return data, ct
}
