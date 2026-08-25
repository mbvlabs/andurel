// Package inertia implements the Inertia v3 protocol for Echo and templ.
package inertia

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io"
	"strings"

	"github.com/a-h/templ"
)

// RootFunc constructs the application's compiled templ document.
type RootFunc func(RootData) templ.Component

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

// PageScript renders v3's non-SSR JSON page script. The JSON is validated and
// every slash is escaped without HTML-entity encoding the script body.
func PageScript(containerID string, pageJSON []byte) templ.Component {
	return templ.ComponentFunc(func(_ context.Context, writer io.Writer) error {
		escaped, err := pageScriptJSON(pageJSON)
		if err != nil {
			return err
		}
		if containerID == "" {
			containerID = "app"
		}
		if _, err := io.WriteString(writer, `<script data-page="`+html.EscapeString(containerID)+`" type="application/json">`); err != nil {
			return err
		}
		if _, err := writer.Write(escaped); err != nil {
			return err
		}
		_, err = io.WriteString(writer, `</script>`)
		return err
	})
}

func pageScriptJSON(pageJSON []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(pageJSON))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("inertia: invalid page JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("inertia: invalid trailing page JSON")
	}
	var normalized bytes.Buffer
	encoder := json.NewEncoder(&normalized)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, fmt.Errorf("inertia: normalize page JSON: %w", err)
	}
	result := bytes.TrimSuffix(normalized.Bytes(), []byte("\n"))
	return bytes.ReplaceAll(result, []byte("/"), []byte(`\/`)), nil
}

// AppMount renders the empty client-rendering mount element.
func AppMount(containerID string) templ.Component {
	return templ.ComponentFunc(func(_ context.Context, writer io.Writer) error {
		if containerID == "" {
			containerID = "app"
		}
		_, err := io.WriteString(writer, `<div id="`+html.EscapeString(containerID)+`"></div>`)
		return err
	})
}

// SSRHead renders trusted head markup returned by the configured local runtime.
func SSRHead(response *SSRResponse) templ.Component {
	return templ.ComponentFunc(func(_ context.Context, writer io.Writer) error {
		if response == nil {
			return nil
		}
		_, err := io.WriteString(writer, strings.Join(response.Head, "\n"))
		return err
	})
}

// SSRBody renders trusted body markup returned by the configured local runtime.
// The body replaces both PageScript and AppMount.
func SSRBody(response *SSRResponse) templ.Component {
	return templ.ComponentFunc(func(_ context.Context, writer io.Writer) error {
		if response == nil {
			return nil
		}
		_, err := io.WriteString(writer, response.Body)
		return err
	})
}
