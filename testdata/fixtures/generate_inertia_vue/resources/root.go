package resources

import (
	"embed"
	"html/template"

)

//go:embed root.html
var files embed.FS

// Root is the Inertia root document. html/template escapes .ProjectName and the
// RootData methods return pre-rendered safe fragments for the page JSON, SSR
// markup, and Vite tags.
var Root = template.Must(template.New("root.html").ParseFS(files, "root.html"))
