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
		{"framework_elements_andurel.tmpl", "internal/andurel/andurel.go"},
		{"framework_elements_routes.tmpl", "internal/andurel/routes.go"},
		{"framework_elements_route_definitions.tmpl", "internal/andurel/route_definitions.go"},
		{"framework_elements_server.tmpl", "internal/andurel/server.go"},
		{"framework_elements_database.tmpl", "internal/andurel/database.go"},
		{"framework_elements_queue.tmpl", "internal/andurel/queue.go"},
		{"framework_elements_render.tmpl", "internal/andurel/render.go"},
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
func (g *TemplateGenerator) RenderFrameworkTemplates(config layout.ScaffoldConfig) (map[string][]byte, error) {
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
	moduleName := config.ProjectName
	if config.Repository != "" {
		moduleName = config.Repository + "/" + config.ProjectName
	}

	return &layout.TemplateData{
		AppName:      config.ProjectName,
		ProjectName:  config.ProjectName,
		ModuleName:   moduleName,
		Database:     config.Database,
		CSSFramework: config.CSSFramework,
		Extensions:   config.Extensions,
	}
}

// renderTemplateToBytes renders a template from the given filesystem and returns the result as bytes
func renderTemplateToBytes(templateFile string, fsys fs.FS, data *layout.TemplateData) ([]byte, error) {
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
