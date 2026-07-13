// Package templates exposes the embedded templates used by code generators.
package templates

import "embed"

//go:embed *.tmpl
var Files embed.FS
