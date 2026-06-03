package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mbvlabs/andurel/generator/templates"
	"github.com/mbvlabs/andurel/pkg/constants"
	"github.com/mbvlabs/andurel/pkg/naming"
	"github.com/spf13/cobra"
)

type emailTemplateData struct {
	PascalName string
	SnakeName  string
}

func newGenerateEmailCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "email NAME",
		Short: "Generate a new email template",
		Long: `Generates a new email template with the given name. Pass the email name
in CamelCase.

This creates a templ email template file in email/ that implements the
Transformer interface and can be used with email.SendTransactional or
email.SendMarketing.`,
		Example: `  andurel generate email WelcomeEmail

      Creates a WelcomeEmail template at email/welcome_email.templ`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if err := chdirToProjectRoot(); err != nil {
				return err
			}

			return withGenerateCleanup(func(_ *cobra.Command, _ []string) error {
				return generateEmail(name)
			})(cmd, args)
		},
	}

	return cmd
}

func generateEmail(name string) error {
	snakeName := naming.ToSnakeCase(name)
	pascalName := naming.ToPascalCase(snakeName)

	emailPath := filepath.Join("email", snakeName+".templ")
	if err := generateEmailFromTemplate(emailPath, emailTemplateData{
		PascalName: pascalName,
		SnakeName:  snakeName,
	}); err != nil {
		return fmt.Errorf("failed to generate email template: %w", err)
	}

	fmt.Printf("Successfully generated email template %s\n", name)
	return nil
}

func generateEmailFromTemplate(outputPath string, data emailTemplateData) error {
	if _, err := os.Stat(outputPath); err == nil {
		return fmt.Errorf("file %s already exists", outputPath)
	}

	content, err := templates.RenderTemplateUsingGlobal("email.tmpl", data)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(outputPath, []byte(content), constants.FilePermissionPrivate); err != nil {
		return err
	}

	return nil
}
