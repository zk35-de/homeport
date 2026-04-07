package api

import "testing"

func TestFaviconSrc(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"http://example.com/favicon.ico", "/api/favicon?url=http%3A%2F%2Fexample.com%2Ffavicon.ico"},
		{"https://example.com/favicon.ico", "/api/favicon?url=https%3A%2F%2Fexample.com%2Ffavicon.ico"},
		{"/api/favicon?url=https%3A%2F%2Fexample.com", "/api/favicon?url=https%3A%2F%2Fexample.com"},
		{"🔗", "🔗"},
		{"", ""},
	}
	for _, tt := range tests {
		got := faviconSrc(tt.in)
		if got != tt.want {
			t.Errorf("faviconSrc(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
