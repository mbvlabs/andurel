package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

var rootCommonCommands = []struct {
	Use         string
	Description string
}{
	{"andurel generate scaffold <Name>", "Create a model, controller, views, and routes"},
	{"andurel db migrate up", "Apply pending SQL migrations"},
	{"andurel run", "Start the development server"},
	{"andurel inspect project", "Show lockfile, adapter, and tool status"},
	{"andurel doctor", "Check project health and generated-file drift"},
}

func renderHumanHelp(cmd *cobra.Command, _ []string) {
	if renderStructuredHelpIfNeeded(cmd) {
		return
	}
	w := cmd.OutOrStdout()
	if cmd.Parent() == nil {
		renderRootHelp(cmd, w)
		return
	}
	if len(availableSubcommands(cmd)) > 0 {
		renderGroupHelp(cmd, w)
		return
	}
	renderLeafHelp(cmd, w)
}

func renderRootHelp(cmd *cobra.Command, w io.Writer) {
	if !isInAndurelProject() {
		fmt.Fprintln(w, "Andurel — space-grade Go framework for humans and agents")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Usage:")
		fmt.Fprintln(w, "  andurel <command> [args...]")
		fmt.Fprintln(w, "  andurel commands [--json|--markdown|--check]")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "You must specify a command:")
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  %-14s %s\n", "new", "Stand up a new Andurel project")
		fmt.Fprintln(w)
		writeDiscoveryHelp(w)
		fmt.Fprintln(w, "Global Flags:")
		fmt.Fprint(w, cmd.PersistentFlags().FlagUsages())
		return
	}

	fmt.Fprintln(w, "Andurel — space-grade Go framework for humans and agents")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  andurel <command> [args...]")
	fmt.Fprintln(w, "  andurel commands [--json|--markdown|--check]")
	fmt.Fprintln(w, "  andurel <group> --help")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Common commands:")
	width := 0
	for _, item := range rootCommonCommands {
		if len(item.Use) > width {
			width = len(item.Use)
		}
	}
	for _, item := range rootCommonCommands {
		fmt.Fprintf(w, "  %-*s  %s\n", width, item.Use, item.Description)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Groups:")
	groups := make([]*cobra.Command, 0)
	leaves := make([]*cobra.Command, 0)
	for _, sub := range availableSubcommands(cmd) {
		if sub.Name() == "new" || sub.Name() == "commands" {
			continue
		}
		if len(availableSubcommands(sub)) > 0 {
			groups = append(groups, sub)
			continue
		}
		leaves = append(leaves, sub)
	}
	writeCommandNameTable(w, groups)
	if len(leaves) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Commands:")
		writeCommandNameTable(w, leaves)
	}
	fmt.Fprintln(w)
	writeDiscoveryHelp(w)
	fmt.Fprintln(w, "Global Flags:")
	fmt.Fprint(w, cmd.PersistentFlags().FlagUsages())
}

func renderGroupHelp(cmd *cobra.Command, w io.Writer) {
	fmt.Fprintln(w, commandHeadline(cmd))
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintf(w, "  %s <command> [args...]\n", cmd.CommandPath())
	fmt.Fprintf(w, "  %s --help\n", cmd.CommandPath())
	fmt.Fprintln(w)
	children := availableSubcommands(cmd)
	if len(children) == 0 {
		return
	}
	fmt.Fprintln(w, "Commands:")
	rows := make([][2]string, 0, len(children))
	maxUse := 0
	for _, child := range children {
		use := child.CommandPath()
		meta, ok := metaFor(child)
		if ok && meta.Args != "" {
			use = strings.TrimSpace(child.CommandPath() + " " + meta.Args)
		}
		if len(use) > maxUse {
			maxUse = len(use)
		}
		rows = append(rows, [2]string{use, commandHeadline(child)})
	}
	for _, row := range rows {
		fmt.Fprintf(w, "  %-*s  %s\n", maxUse, row[0], row[1])
	}
	if cmd.HasAvailableLocalFlags() {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Flags:")
		fmt.Fprint(w, cmd.LocalFlags().FlagUsages())
	}
	if cmd.HasAvailableInheritedFlags() {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Global Flags:")
		fmt.Fprint(w, cmd.InheritedFlags().FlagUsages())
	}
}

func renderLeafHelp(cmd *cobra.Command, w io.Writer) {
	meta, ok := metaFor(cmd)
	fmt.Fprintln(w, commandHeadline(cmd))
	fmt.Fprintln(w)
	if ok {
		writeBulletSection(w, "When to use", meta.WhenToUse)
		writeBulletSection(w, "When not to use", meta.WhenNotToUse)
		writeBulletSection(w, "Prerequisites", meta.Prerequisites)
	} else if strings.TrimSpace(cmd.Long) != "" && cmd.Long != cmd.Short {
		fmt.Fprintln(w, cmd.Long)
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintf(w, "  %s\n", cmd.UseLine())
	fmt.Fprintln(w)
	examples := commandExamples(cmd, meta, ok)
	if len(examples) > 0 {
		fmt.Fprintln(w, "Examples:")
		for _, example := range examples {
			fmt.Fprintf(w, "  %s\n", example)
		}
		fmt.Fprintln(w)
	}
	if ok {
		writeBulletSection(w, "Next", meta.Next)
	}
	if cmd.HasAvailableLocalFlags() {
		fmt.Fprintln(w, "Flags:")
		fmt.Fprint(w, cmd.LocalFlags().FlagUsages())
	}
	if cmd.HasAvailableInheritedFlags() {
		fmt.Fprintln(w, "Global Flags:")
		fmt.Fprint(w, cmd.InheritedFlags().FlagUsages())
	}
}

func commandHeadline(cmd *cobra.Command) string {
	if meta, ok := metaFor(cmd); ok && strings.TrimSpace(meta.Summary) != "" {
		return meta.Summary
	}
	if strings.TrimSpace(cmd.Short) != "" {
		return cmd.Short
	}
	return strings.TrimSpace(cmd.Long)
}

func commandExamples(cmd *cobra.Command, meta CommandMeta, ok bool) []string {
	if ok && len(meta.Examples) > 0 {
		return meta.Examples
	}
	example := strings.TrimSpace(cmd.Example)
	if example == "" {
		return nil
	}
	return []string{example}
}

func writeBulletSection(w io.Writer, title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(w, "%s:\n", title)
	for _, item := range items {
		fmt.Fprintf(w, "  - %s\n", item)
	}
	fmt.Fprintln(w)
}

func writeDiscoveryHelp(w io.Writer) {
	fmt.Fprintln(w, "Discovery:")
	fmt.Fprintln(w, "  andurel commands --json       Machine-readable catalog")
	fmt.Fprintln(w, "  andurel commands --markdown   Markdown command table")
	fmt.Fprintln(w, "  andurel commands --check      Validate metadata completeness")
	fmt.Fprintln(w)
}

func writeCommandNameTable(w io.Writer, commands []*cobra.Command) {
	maxName := 0
	for _, command := range commands {
		if len(command.Name()) > maxName {
			maxName = len(command.Name())
		}
	}
	for _, command := range commands {
		fmt.Fprintf(w, "  %-*s  %s\n", maxName, command.Name(), commandHeadline(command))
	}
}
