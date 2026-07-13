// Package templates exposes the embedded templates used to scaffold Andurel projects.
package templates

import "embed"

//go:embed *.tmpl
var Files embed.FS
