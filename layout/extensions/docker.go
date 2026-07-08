package extensions

import "fmt"

// Docker adds Docker build files to a scaffolded project.
type Docker struct{}

// Name returns the extension name used in lock files and CLI flags.
func (d Docker) Name() string {
	return "docker"
}

// Apply renders Docker templates into the target project.
func (d Docker) Apply(ctx *Context) error {
	if ctx == nil || ctx.Data == nil {
		return fmt.Errorf("docker: context or data is nil")
	}

	if err := d.renderTemplates(ctx); err != nil {
		return fmt.Errorf("docker: failed to render templates: %w", err)
	}

	return nil
}

// Dependencies returns extension names that must be applied first.
func (d Docker) Dependencies() []string {
	return nil
}

func (d Docker) renderTemplates(ctx *Context) error {
	templates := map[string]string{
		"Dockerfile.tmpl":   "Dockerfile",
		"dockerignore.tmpl": ".dockerignore",
	}

	for tmpl, target := range templates {
		templatePath := fmt.Sprintf("templates/docker/%s", tmpl)
		if err := ctx.ProcessTemplate(templatePath, target, nil); err != nil {
			return fmt.Errorf("failed to process %s: %w", tmpl, err)
		}
	}

	return nil
}
