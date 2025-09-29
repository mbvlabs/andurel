package templates

import "embed"

//go:embed *.tmpl recipes/**/*.tmpl
var Files embed.FS
