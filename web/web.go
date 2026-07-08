// Package web embeds the panel's HTML templates and static assets so the whole
// panel ships inside the single binary with no external files.
package web

import "embed"

//go:embed templates/*.html static/*
var FS embed.FS
