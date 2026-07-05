package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mbvlabs/andurel/cli/output"
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
	var dryRun bool
	var diff bool

	cmd := &cobra.Command{
		Use:     "email NAME",
		Aliases: []string{"e"},
		Short:   "Generate a new email template",
		Long: `Generates a new email template with the given name. Pass the email name
in CamelCase.

This creates a templ email template file in email/ that implements the
Transformer interface and can be used with email.SendTransactional or
email.SendMarketing.`,
		Example: `  andurel generate email WelcomeEmail

      Creates a WelcomeEmail template at email/welcome_email.templ`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if len(args) > 1 {
				return fmt.Errorf("too many arguments: email takes exactly 1 argument (the email name)")
			}
			name := args[0]

			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}

			return runMutation(cmd, mutationOptions{
				Action:   "generate email",
				Resource: name,
				RootDir:  rootDir,
				DryRun:   dryRun,
				Diff:     diff,
				Breadcrumbs: []output.Breadcrumb{
					{Command: "andurel views --json", Description: "Inspect generated templates"},
					{Command: "andurel doctor", Description: "Verify project health"},
				},
				Run: func(rootDir string) error {
					return withGenerateCleanup(func(_ *cobra.Command, _ []string) error {
						return generateEmail(name)
					})(cmd, args)
				},
			})
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview file changes without applying")
	cmd.Flags().BoolVar(&diff, "diff", false, "Include a text diff preview in structured output")

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
