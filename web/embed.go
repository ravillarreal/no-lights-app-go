// Package web embeds the server-rendered frontend assets into the binary so
// the API server is a single self-contained executable.
package web

import "embed"

//go:embed templates/* static/*
var FS embed.FS
