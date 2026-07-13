// Package templates exposes the embedded templates used by code generators.
package templates

import "embed"

// Files contains the templates used by code generators.
//
//go:embed *.tmpl
var Files embed.FS
