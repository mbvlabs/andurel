// Package templates exposes the embedded templates used to scaffold Andurel projects.
package templates

import "embed"

// Files contains the templates used to scaffold Andurel projects.
//
//go:embed *.tmpl
var Files embed.FS
