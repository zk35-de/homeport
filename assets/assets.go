// Package assets embeds the static files and HTML templates.
package assets

import "embed"

//go:embed templates static
var FS embed.FS
