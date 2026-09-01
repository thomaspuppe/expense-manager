// Package web embeds the PWA assets and HTML templates into the binary.
package web

import "embed"

//go:embed *.html assets
var FS embed.FS
