// Package inertia implements the Inertia v3 protocol for anduin.
package inertia

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io"
	"strings"
)

// RootFunc renders a complete root document. WithRoot uses an html/template
// root; WithRootFunc adapts a custom document renderer (for example a test
// double).
type RootFunc func(w io.Writer, data RootData) error

// RootData is passed to the configured root document.
type RootData struct {
	Page        Page
	PageJSON    []byte
	ContainerID string
	ProjectName string
	Environment string
	ViteHead    template.HTML
	ViteBody    template.HTML
	SSR         *SSRResponse
}

// PageScript renders the non-SSR JSON page script as a pre-rendered safe
// fragment for html/template root documents. The JSON is validated,
// HTML-escaped, and has every slash escaped so a prop cannot close the script
// element or open an HTML comment.
func (data RootData) PageScript() (template.HTML, error) {
	decoder := json.NewDecoder(bytes.NewReader(data.PageJSON))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", fmt.Errorf("inertia: invalid page JSON: %w", err)
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return "", fmt.Errorf("inertia: invalid trailing page JSON")
	}

	var normalized bytes.Buffer
	encoder := json.NewEncoder(&normalized)
	if err := encoder.Encode(value); err != nil {
		return "", fmt.Errorf("inertia: normalize page JSON: %w", err)
	}

	escaped := bytes.ReplaceAll(
		bytes.TrimSuffix(normalized.Bytes(), []byte("\n")),
		[]byte("/"),
		[]byte(`\/`),
	)

	return template.HTML(
		`<script data-page="` + html.EscapeString(data.ContainerID) + `" type="application/json">` +
			string(escaped) + `</script>`,
	), nil
}

// AppMount renders the empty client-rendering mount element as a pre-rendered
// safe fragment.
func (data RootData) AppMount() template.HTML {
	return template.HTML(`<div id="` + html.EscapeString(data.ContainerID) + `"></div>`)
}

// SSRHead renders trusted head markup returned by the configured local runtime
// as a pre-rendered safe fragment.
func (data RootData) SSRHead() template.HTML {
	if data.SSR == nil {
		return ""
	}
	return template.HTML(strings.Join(data.SSR.Head, "\n"))
}

// SSRBody renders trusted body markup returned by the configured local runtime
// as a pre-rendered safe fragment. The body replaces both PageScript and
// AppMount.
func (data RootData) SSRBody() template.HTML {
	if data.SSR == nil {
		return ""
	}
	return template.HTML(data.SSR.Body)
}
