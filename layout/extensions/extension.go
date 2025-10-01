package extensions

import "embed"

//go:embed */templates/*.tmpl
var Files embed.FS
