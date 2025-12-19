package extensions

import "fmt"

type Docker struct{}

func (d Docker) Name() string {
	return "docker"
}

func (d Docker) Apply(ctx *Context) error {
	if ctx == nil || ctx.Data == nil {
		return fmt.Errorf("docker: context or data is nil")
	}

	if err := d.renderTemplates(ctx); err != nil {
		return fmt.Errorf("docker: failed to render templates: %w", err)
	}

	return nil
}

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
