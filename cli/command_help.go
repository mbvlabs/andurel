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
		helpPrintln(w, "Andurel — space-grade Go framework for humans and agents")
		helpPrintln(w)
		helpPrintln(w, "Usage:")
		helpPrintln(w, "  andurel <command> [args...]")
		helpPrintln(w, "  andurel commands [--json|--markdown|--check]")
		helpPrintln(w)
		helpPrintln(w, "You must specify a command:")
		helpPrintln(w)
		helpPrintf(w, "  %-14s %s\n", "new", "Stand up a new Andurel project")
		helpPrintln(w)
		writeDiscoveryHelp(w)
		helpPrintln(w, "Global Flags:")
		helpPrint(w, cmd.PersistentFlags().FlagUsages())
		return
	}

	helpPrintln(w, "Andurel — space-grade Go framework for humans and agents")
	helpPrintln(w)
	helpPrintln(w, "Usage:")
	helpPrintln(w, "  andurel <command> [args...]")
	helpPrintln(w, "  andurel commands [--json|--markdown|--check]")
	helpPrintln(w, "  andurel <group> --help")
	helpPrintln(w)
	helpPrintln(w, "Common commands:")
	width := 0
	for _, item := range rootCommonCommands {
		if len(item.Use) > width {
			width = len(item.Use)
		}
	}
	for _, item := range rootCommonCommands {
		helpPrintf(w, "  %-*s  %s\n", width, item.Use, item.Description)
	}
	helpPrintln(w)
	helpPrintln(w, "Groups:")
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
		helpPrintln(w)
		helpPrintln(w, "Commands:")
		writeCommandNameTable(w, leaves)
	}
	helpPrintln(w)
	writeDiscoveryHelp(w)
	helpPrintln(w, "Global Flags:")
	helpPrint(w, cmd.PersistentFlags().FlagUsages())
}

func renderGroupHelp(cmd *cobra.Command, w io.Writer) {
	helpPrintln(w, commandHeadline(cmd))
	helpPrintln(w)
	helpPrintln(w, "Usage:")
	helpPrintf(w, "  %s <command> [args...]\n", cmd.CommandPath())
	helpPrintf(w, "  %s --help\n", cmd.CommandPath())
	helpPrintln(w)
	children := availableSubcommands(cmd)
	if len(children) == 0 {
		return
	}
	helpPrintln(w, "Commands:")
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
		helpPrintf(w, "  %-*s  %s\n", maxUse, row[0], row[1])
	}
	if cmd.HasAvailableLocalFlags() {
		helpPrintln(w)
		helpPrintln(w, "Flags:")
		helpPrint(w, cmd.LocalFlags().FlagUsages())
	}
	if cmd.HasAvailableInheritedFlags() {
		helpPrintln(w)
		helpPrintln(w, "Global Flags:")
		helpPrint(w, cmd.InheritedFlags().FlagUsages())
	}
}

func renderLeafHelp(cmd *cobra.Command, w io.Writer) {
	meta, ok := metaFor(cmd)
	helpPrintln(w, commandHeadline(cmd))
	helpPrintln(w)
	if ok {
		writeBulletSection(w, "When to use", meta.WhenToUse)
		writeBulletSection(w, "When not to use", meta.WhenNotToUse)
		writeBulletSection(w, "Prerequisites", meta.Prerequisites)
	} else if strings.TrimSpace(cmd.Long) != "" && cmd.Long != cmd.Short {
		helpPrintln(w, cmd.Long)
		helpPrintln(w)
	}
	helpPrintln(w, "Usage:")
	helpPrintf(w, "  %s\n", cmd.UseLine())
	helpPrintln(w)
	examples := commandExamples(cmd, meta, ok)
	if len(examples) > 0 {
		helpPrintln(w, "Examples:")
		for _, example := range examples {
			helpPrintf(w, "  %s\n", example)
		}
		helpPrintln(w)
	}
	if ok {
		writeBulletSection(w, "Next", meta.Next)
	}
	if cmd.HasAvailableLocalFlags() {
		helpPrintln(w, "Flags:")
		helpPrint(w, cmd.LocalFlags().FlagUsages())
	}
	if cmd.HasAvailableInheritedFlags() {
		helpPrintln(w, "Global Flags:")
		helpPrint(w, cmd.InheritedFlags().FlagUsages())
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
	helpPrintf(w, "%s:\n", title)
	for _, item := range items {
		helpPrintf(w, "  - %s\n", item)
	}
	helpPrintln(w)
}

func writeDiscoveryHelp(w io.Writer) {
	helpPrintln(w, "Discovery:")
	helpPrintln(w, "  andurel commands --json       Machine-readable catalog")
	helpPrintln(w, "  andurel commands --markdown   Markdown command table")
	helpPrintln(w, "  andurel commands --check      Validate metadata completeness")
	helpPrintln(w)
}

func writeCommandNameTable(w io.Writer, commands []*cobra.Command) {
	maxName := 0
	for _, command := range commands {
		if len(command.Name()) > maxName {
			maxName = len(command.Name())
		}
	}
	for _, command := range commands {
		helpPrintf(w, "  %-*s  %s\n", maxName, command.Name(), commandHeadline(command))
	}
}

func helpPrint(w io.Writer, a ...any) {
	_, _ = fmt.Fprint(w, a...)
}

func helpPrintf(w io.Writer, format string, a ...any) {
	_, _ = fmt.Fprintf(w, format, a...)
}

func helpPrintln(w io.Writer, a ...any) {
	_, _ = fmt.Fprintln(w, a...)
}
