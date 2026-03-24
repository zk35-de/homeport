// Package assets embeds the static files and HTML templates.
package assets

import "embed"

//go:embed templates static i18n
var FS embed.FS
