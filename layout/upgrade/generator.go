package upgrade

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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
func GetFrameworkTemplates(config *layout.ScaffoldConfig) []FrameworkTemplate {
	files := layout.GetInternalFrameworkFiles(config)
	templates := make([]FrameworkTemplate, 0, len(files))
	for _, file := range files {
		templates = append(templates, FrameworkTemplate{
			TemplateName: file.TemplateName,
			TargetPath:   file.TargetPath,
		})
	}

	return templates
}

// TemplateGenerator generates template artifacts.
type TemplateGenerator struct {
	targetVersion string
}

// NewTemplateGenerator creates a new template generator.
func NewTemplateGenerator(targetVersion string) *TemplateGenerator {
	return &TemplateGenerator{
		targetVersion: targetVersion,
	}
}

// RenderFrameworkTemplates renders all framework element templates and returns
// a map of file paths to their rendered content
func (g *TemplateGenerator) RenderFrameworkTemplates(
	projectRoot string,
	config layout.ScaffoldConfig,
	extensions []string,
) (map[string][]byte, error) {
	modulePath, err := resolveModulePath(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve module path: %w", err)
	}

	templateData := g.buildTemplateData(config, modulePath, extensions)
	result := make(map[string][]byte)

	frameworkTemplates := GetFrameworkTemplates(&config)

	for _, ft := range frameworkTemplates {
		content, err := renderTemplateToBytes(ft.TemplateName, templates.Files, templateData)
		if err != nil {
			return nil, fmt.Errorf("failed to render %s: %w", ft.TemplateName, err)
		}

		result[ft.TargetPath] = content
	}

	return result, nil
}

// buildTemplateData constructs the template data from scaffold config and go.mod.
func (g *TemplateGenerator) buildTemplateData(
	config layout.ScaffoldConfig,
	modulePath string,
	extensions []string,
) *layout.TemplateData {
	frameworkVersion := strings.TrimSpace(g.targetVersion)
	if frameworkVersion == "" {
		frameworkVersion = "dev"
	}

	return &layout.TemplateData{
		AppName:          config.ProjectName,
		ProjectName:      config.ProjectName,
		ModuleName:       modulePath,
		Database:         config.Database,
		Extensions:       extensions,
		RunToolVersion:   layout.GetRunToolVersion(),
		FrameworkVersion: frameworkVersion,
		Inertia:          config.Inertia,
	}
}

func resolveModulePath(projectRoot string) (string, error) {
	goModPath := filepath.Join(projectRoot, "go.mod")

	file, err := os.Open(goModPath)
	if err != nil {
		return "", fmt.Errorf("failed to open go.mod: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return "", fmt.Errorf("invalid module declaration in go.mod")
			}

			return fields[1], nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read go.mod: %w", err)
	}

	return "", fmt.Errorf("module declaration not found in go.mod")
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
