package cli

import (
	"fmt"

	"github.com/mbvlabs/andurel/cli/output"
	"github.com/mbvlabs/andurel/layout"
	"github.com/spf13/cobra"
)

func newToolCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tool",
		Aliases: []string{"tools", "t"},
		Short:   "Manage project tools and binaries",
		Long: `Manage CLI tools and binaries used by your Andurel project.

Tools are defined in andurel.lock and downloaded to bin/. Use the
subcommands below to sync, configure, or run tools.`,
		Example: `  andurel tool sync
  andurel tool list --json
  andurel tool set-version templ 0.3.977
  andurel tool dblab
  andurel tool mailpit`,
	}
	setAgentMetadata(
		cmd,
		"introspection",
		"Group for lockfile tools. Use tool list, tool sync, and other subcommands.",
	)

	cmd.AddCommand(newSyncCommand())
	cmd.AddCommand(newSetVersionCommand())
	cmd.AddCommand(newDblabCommand())
	cmd.AddCommand(newMailpitCommand())
	cmd.AddCommand(newToolListCommand())

	return cmd
}

func newToolListCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls", "status"},
		Short:   "List project tool status",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}
			lock, err := layout.ReadLockFile(rootDir)
			if err != nil {
				return err
			}
			return renderToolReport(cmd, toolInfos(rootDir, lock))
		},
	}
}

func renderToolReport(cmd *cobra.Command, tools []toolInfo) error {
	summary := toolListSummary(tools)
	opts, err := output.ParseOptions(cmd)
	if err != nil {
		return err
	}
	if output.UsesStructuredOutput(opts) {
		return output.OK(cmd, tools, summary)
	}
	if opts.Quiet {
		return nil
	}

	if _, err := fmt.Fprintln(cmd.OutOrStdout(), summary); err != nil {
		return err
	}
	if len(tools) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(cmd.OutOrStdout()); err != nil {
		return err
	}
	width := toolNameWidth(tools)
	for _, tool := range tools {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), formatToolLine(tool, width)); err != nil {
			return err
		}
	}
	return nil
}

func toolListSummary(tools []toolInfo) string {
	if len(tools) == 0 {
		return "No tools pinned in andurel.lock"
	}
	installed := 0
	for _, tool := range tools {
		if tool.Installed {
			installed++
		}
	}
	if installed == len(tools) {
		if len(tools) == 1 {
			return "1 tool is installed"
		}
		return fmt.Sprintf("All %d tools are installed", len(tools))
	}
	if installed == 1 {
		return fmt.Sprintf("1 of %d tools installed", len(tools))
	}
	return fmt.Sprintf("%d of %d tools installed", installed, len(tools))
}

func toolNameWidth(tools []toolInfo) int {
	width := 0
	for _, tool := range tools {
		if len(tool.Name) > width {
			width = len(tool.Name)
		}
	}
	return width
}

func formatToolLine(tool toolInfo, width int) string {
	name := fmt.Sprintf("%-*s", width, tool.Name)
	version := tool.Version
	if version == "" {
		version = "-"
	}
	status := "missing"
	if tool.Installed {
		status = "installed"
	}
	return fmt.Sprintf("  %s  %s  %s", name, version, status)
}
