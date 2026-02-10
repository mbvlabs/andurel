package upgrade

import (
	"bytes"
	"fmt"
	"io/fs"
	"strings"
	"text/template"

	"github.com/mbvlabs/andurel/layout"
	"github.com/mbvlabs/andurel/layout/templates"
)

// FrameworkTemplate represents a framework element template and its target path
type FrameworkTemplate struct {
	TemplateName string
	TargetPath   string
}

// GetFrameworkTemplates returns the list of framework element templates
// These are the only files that get upgraded when running andurel upgrade
func GetFrameworkTemplates() []FrameworkTemplate {
	return []FrameworkTemplate{
		{"framework_elements_renderer_fragments.tmpl", "internal/renderer/fragments.go"},
		{"framework_elements_renderer_render.tmpl", "internal/renderer/render.go"},

		{"framework_elements_routing_definitions.tmpl", "internal/routing/definitions.go"},
		{"framework_elements_routing_routes.tmpl", "internal/routing/routes.go"},
		{"framework_elements_routing_routes_test.tmpl", "internal/routing/routes_test.go"},

		{"framework_elements_server_server.tmpl", "internal/server/server.go"},

		{"framework_elements_storage_psql.tmpl", "internal/storage/psql.go"},
		{"framework_elements_storage_queue.tmpl", "internal/storage/queue.go"},

		{"framework_elements_hypermedia_broadcaster.tmpl", "internal/hypermedia/broadcaster.go"},
		{"framework_elements_hypermedia_core.tmpl", "internal/hypermedia/core.go"},
		{"framework_elements_hypermedia_helpers.tmpl", "internal/hypermedia/helpers.go"},
		{"framework_elements_hypermedia_signals.tmpl", "internal/hypermedia/signals.go"},
		{"framework_elements_hypermedia_sse.tmpl", "internal/hypermedia/sse.go"},

		{"tw_internal_ui_utils.tmpl", "internal/ui/utils.go"},
		{"tw_views_components_button.tmpl", "internal/ui/button.templ"},
		{"tw_views_components_card.tmpl", "internal/ui/card.templ"},
		{"tw_views_components_label.tmpl", "internal/ui/label.templ"},
		{"tw_views_components_separator.tmpl", "internal/ui/separator.templ"},
		{"tw_views_components_table.tmpl", "internal/ui/table.templ"},
		{"tw_views_components_form.tmpl", "internal/ui/form.templ"},
		{"tw_views_components_checkbox.tmpl", "internal/ui/checkbox.templ"},
		{"tw_views_components_input.tmpl", "internal/ui/input.templ"},
		{"tw_views_components_datepicker.tmpl", "internal/ui/datepicker.templ"},
	}
}

type TemplateGenerator struct {
	targetVersion string
}

func NewTemplateGenerator(targetVersion string) *TemplateGenerator {
	return &TemplateGenerator{
		targetVersion: targetVersion,
	}
}

// RenderFrameworkTemplates renders all framework element templates and returns
// a map of file paths to their rendered content
func (g *TemplateGenerator) RenderFrameworkTemplates(
	config layout.ScaffoldConfig,
) (map[string][]byte, error) {
	templateData := g.buildTemplateData(config)
	result := make(map[string][]byte)

	frameworkTemplates := GetFrameworkTemplates()

	for _, ft := range frameworkTemplates {
		content, err := renderTemplateToBytes(ft.TemplateName, templates.Files, templateData)
		if err != nil {
			return nil, fmt.Errorf("failed to render %s: %w", ft.TemplateName, err)
		}

		result[ft.TargetPath] = content
	}

	return result, nil
}

// buildTemplateData constructs the template data from scaffold config
func (g *TemplateGenerator) buildTemplateData(config layout.ScaffoldConfig) *layout.TemplateData {
	return &layout.TemplateData{
		AppName:        config.ProjectName,
		ProjectName:    config.ProjectName,
		ModuleName:     config.ProjectName,
		Database:       config.Database,
		CSSFramework:   config.CSSFramework,
		Extensions:     config.Extensions,
		RunToolVersion: layout.GetRunToolVersion(),
	}
}

// renderTemplateToBytes renders a template from the given filesystem and returns the result as bytes
func renderTemplateToBytes(
	templateFile string,
	fsys fs.FS,
	data *layout.TemplateData,
) ([]byte, error) {
	content, err := fs.ReadFile(fsys, templateFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read template %s: %w", templateFile, err)
	}

	funcMap := template.FuncMap{
		"lower": strings.ToLower,
	}

	tmpl, err := template.New(templateFile).
		Funcs(funcMap).
		Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template %s: %w", templateFile, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to execute template %s: %w", templateFile, err)
	}

	return buf.Bytes(), nil
}
