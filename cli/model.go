package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mbvlabs/andurel/generator"
	"github.com/spf13/cobra"
)

func newModelRootCommand() *cobra.Command {
	var tableName string
	var skipFactory bool

	cmd := &cobra.Command{
		Use:   "model <name> <command>",
		Short: "Model management commands",
		Long:  "Manage resource models.\n\n<ResourceName> is the associated model name used for generation.",
		Example: `  model User create
  model User create --table-name=accounts
  model User create --skip-factory
  model User update`,
	}

	setStandardHelp(cmd,
		helpCommand{
			Use:         "model <ResourceName> create",
			Description: "creates a resource model",
		},
		helpCommand{
			Use:         "model <ResourceName> update",
			Description: "updates a resource model from migrations",
		},
	)

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 {
			return cmd.Help()
		}
		if len(args) > 2 {
			return fmt.Errorf("too many arguments\nRun 'andurel model --help' for usage")
		}
		name := args[0]
		switch args[1] {
		case "create":
			if err := chdirToProjectRoot(); err != nil {
				return err
			}
			return withGenerateCleanup(func(_ *cobra.Command, _ []string) error {
				gen, err := generator.New()
				if err != nil {
					return err
				}
				return gen.GenerateModel(name, tableName, skipFactory)
			})(cmd, args)
		case "update":
			if err := chdirToProjectRoot(); err != nil {
				return err
			}
			yes, _ := cmd.Flags().GetBool("yes")
			return runModelUpdate(name, yes)
		default:
			return fmt.Errorf("unknown model command %q\nRun 'andurel model --help' for usage", args[1])
		}
	}

	cmd.Flags().StringVar(&tableName, "table-name", "", "Override the default table name (defaults to plural form of model name)")
	cmd.Flags().BoolVar(&skipFactory, "skip-factory", false, "Skip factory generation")
	cmd.Flags().Bool("yes", false, "Apply changes without prompting for confirmation")

	return cmd
}

func runModelUpdate(resourceName string, autoApply bool) error {
	gen, err := generator.New()
	if err != nil {
		return err
	}

	result, err := gen.UpdateModel(resourceName)
	if err != nil {
		return err
	}

	if !result.HasChanges {
		fmt.Println("No changes — model struct is already up to date.")
		return nil
	}

	diff, err := result.Diff()
	if err != nil {
		return fmt.Errorf("failed to compute diff: %w", err)
	}

	fmt.Printf("Changes to %s:\n\n", result.ModelPath)
	printColoredDiff(diff)
	fmt.Println()

	if !autoApply {
		confirmed, err := confirmModelApply()
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Aborted.")
			return nil
		}
	}

	if err := gen.ApplyModelUpdate(result); err != nil {
		return err
	}

	fmt.Printf("Updated %s\n", result.ModelPath)
	return nil
}

func confirmModelApply() (bool, error) {
	fmt.Print("Apply these changes? [y/N] ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

func printColoredDiff(diff string) {
	for line := range strings.SplitSeq(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			fmt.Println(line)
		case strings.HasPrefix(line, "+"):
			fmt.Printf("\033[32m%s\033[0m\n", line)
		case strings.HasPrefix(line, "-"):
			fmt.Printf("\033[31m%s\033[0m\n", line)
		case strings.HasPrefix(line, "@@"):
			fmt.Printf("\033[36m%s\033[0m\n", line)
		default:
			fmt.Println(line)
		}
	}
}
