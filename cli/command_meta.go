package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/spf13/cobra"
)

// CommandMeta is the canonical per-command documentation for humans and agents.
type CommandMeta struct {
	Group            string   `json:"group,omitempty"`
	Summary          string   `json:"summary"`
	Args             string   `json:"args,omitempty"`
	WhenToUse        []string `json:"when_to_use,omitempty"`
	WhenNotToUse     []string `json:"when_not_to_use,omitempty"`
	Prerequisites    []string `json:"prerequisites,omitempty"`
	Next             []string `json:"next,omitempty"`
	Examples         []string `json:"examples,omitempty"`
	Mutating         bool     `json:"mutating"`
	Destructive      bool     `json:"destructive,omitempty"`
	RequiresProject  bool     `json:"requires_project,omitempty"`
	SupportsDryRun   bool     `json:"supports_dry_run,omitempty"`
	SkipDryRunCheck  bool     `json:"skip_dry_run_check,omitempty"`
	Prompts          bool     `json:"prompts,omitempty"`
	InteractiveFlags []string `json:"interactive_flags,omitempty"`
}

type catalogRecord struct {
	CommandMeta
	Name    string          `json:"name"`
	Path    string          `json:"path"`
	Aliases []string        `json:"aliases,omitempty"`
	Usage   string          `json:"usage"`
	Flags   []flagDiscovery `json:"flags,omitempty"`
}

type commandsDiscoveryPayload struct {
	commandDiscovery
	Catalog []catalogRecord `json:"catalog"`
}

type commandMetaCheckReport struct {
	OK       bool     `json:"ok"`
	Failures []string `json:"failures,omitempty"`
}

var commandMetaByCmd = map[*cobra.Command]CommandMeta{}

func registerMeta(cmd *cobra.Command, meta CommandMeta) {
	if cmd == nil {
		return
	}
	commandMetaByCmd[cmd] = meta
	notes := strings.Join(meta.WhenToUse, " ")
	if notes == "" {
		notes = meta.Summary
	}
	category := meta.Group
	if category == "" {
		category = "root"
	}
	setAgentMetadata(cmd, category, notes)
}

func metaFor(cmd *cobra.Command) (CommandMeta, bool) {
	if cmd == nil {
		return CommandMeta{}, false
	}
	meta, ok := commandMetaByCmd[cmd]
	return meta, ok
}

func groupFromCommand(cmd *cobra.Command) string {
	if cmd == nil || cmd.Parent() == nil {
		return ""
	}
	current := cmd
	for current.Parent() != nil && current.Parent().Parent() != nil {
		current = current.Parent()
	}
	return current.Name()
}

func walkPublicCommands(root *cobra.Command) []*cobra.Command {
	commands := make([]*cobra.Command, 0)
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		if cmd != root && (!cmd.IsAvailableCommand() || cmd.Hidden) {
			return
		}
		commands = append(commands, cmd)
		for _, sub := range availableSubcommands(cmd) {
			walk(sub)
		}
	}
	walk(root)
	return commands
}

func flattenCatalog(root *cobra.Command) []catalogRecord {
	commands := walkPublicCommands(root)
	records := make([]catalogRecord, 0, len(commands))
	for _, cmd := range commands {
		meta, _ := metaFor(cmd)
		records = append(records, catalogRecord{
			CommandMeta: meta,
			Name:        cmd.Name(),
			Path:        cmd.CommandPath(),
			Aliases:     append([]string(nil), cmd.Aliases...),
			Usage:       cmd.UseLine(),
			Flags:       discoverFlags(cmd.NonInheritedFlags()),
		})
	}
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].Path < records[j].Path
	})
	return records
}

func registerAllCommandMeta(root *cobra.Command) {
	commandMetaByCmd = map[*cobra.Command]CommandMeta{}
	for _, cmd := range walkPublicCommands(root) {
		spec, ok := commandMetaSpecs[cmd.CommandPath()]
		if !ok {
			continue
		}
		spec.Group = groupFromCommand(cmd)
		registerMeta(cmd, spec)
	}
}

func checkCommandMetadata(root *cobra.Command) commandMetaCheckReport {
	failures := make([]string, 0)
	seen := map[string]bool{}
	for _, cmd := range walkPublicCommands(root) {
		path := cmd.CommandPath()
		seen[path] = true
		meta, ok := metaFor(cmd)
		if !ok {
			failures = append(failures, path+": missing CommandMeta")
			continue
		}
		if strings.TrimSpace(meta.Summary) == "" {
			failures = append(failures, path+": missing summary")
		}
		if len(meta.WhenToUse) == 0 {
			failures = append(failures, path+": missing when_to_use")
		}
		if len(meta.Examples) == 0 {
			failures = append(failures, path+": missing examples")
		}
		if meta.Mutating && !meta.SupportsDryRun && !meta.SkipDryRunCheck {
			failures = append(
				failures,
				path+": mutating command must support dry-run or skip the check",
			)
		}
		if meta.Prompts && len(meta.InteractiveFlags) == 0 {
			failures = append(failures, path+": prompts without interactive_flags")
		}
		expectedGroup := groupFromCommand(cmd)
		if meta.Group != expectedGroup {
			failures = append(
				failures,
				fmt.Sprintf(
					"%s: group %q does not match cobra parent %q",
					path,
					meta.Group,
					expectedGroup,
				),
			)
		}
		if cmd.Parent() != nil && cmd.Parent().Name() == "generate" {
			switch cmd.Name() {
			case "view", "views", "queries", "routes", "payloads", "factory", "factories":
				failures = append(
					failures,
					path+": derived-file command must not live under generate",
				)
			}
		}
	}
	for path := range commandMetaSpecs {
		if !seen[path] {
			failures = append(failures, path+": CommandMeta registered for unknown command")
		}
	}
	failures = append(failures, checkGeneratedDocs(root)...)
	sort.Strings(failures)
	return commandMetaCheckReport{OK: len(failures) == 0, Failures: failures}
}

func findAndurelFrameworkRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if isAndurelFrameworkRoot(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func isAndurelFrameworkRoot(dir string) bool {
	_, catalogErr := os.Stat(
		filepath.Join(dir, "skills", "andurel", "references", "cli-catalog.md"),
	)
	_, agentsErr := os.Stat(filepath.Join(dir, "layout", "templates", "agents.tmpl"))
	_, readmeErr := os.Stat(filepath.Join(dir, "layout", "templates", "readme.tmpl"))
	return catalogErr == nil && agentsErr == nil && readmeErr == nil
}

func checkGeneratedDocs(root *cobra.Command) []string {
	frameworkRoot := findAndurelFrameworkRoot()
	if frameworkRoot == "" {
		return nil
	}
	failures := make([]string, 0)
	catalogPath := filepath.Join(frameworkRoot, "skills", "andurel", "references", "cli-catalog.md")
	agentsPath := filepath.Join(frameworkRoot, "layout", "templates", "agents.tmpl")
	readmePath := filepath.Join(frameworkRoot, "layout", "templates", "readme.tmpl")
	catalogBody, err := os.ReadFile(catalogPath)
	if err != nil {
		return []string{"skills/andurel/references/cli-catalog.md: " + err.Error()}
	}
	agentsBody, err := os.ReadFile(agentsPath)
	if err != nil {
		return []string{"layout/templates/agents.tmpl: " + err.Error()}
	}
	readmeBody, err := os.ReadFile(readmePath)
	if err != nil {
		return []string{"layout/templates/readme.tmpl: " + err.Error()}
	}
	catalogText := string(catalogBody)
	agentsText := string(agentsBody)
	readmeText := string(readmeBody)
	for _, record := range flattenCatalog(root) {
		needle := "`" + record.Path + "`"
		if !strings.Contains(catalogText, needle) {
			failures = append(failures, "skills/andurel/references/cli-catalog.md: missing "+needle)
		}
	}
	for _, needle := range []string{
		"andurel commands --json",
		"andurel inspect project --json",
		"andurel doctor --json",
	} {
		if !strings.Contains(agentsText, needle) {
			failures = append(failures, "layout/templates/agents.tmpl: missing "+needle)
		}
	}
	for _, retired := range []string{"andurel migration", "andurel app console"} {
		if strings.Contains(readmeText, retired) {
			failures = append(failures, "layout/templates/readme.tmpl: still advertises "+retired)
		}
	}
	for _, needle := range []string{"andurel generate migration", "andurel db migrate", "andurel db console"} {
		if !strings.Contains(readmeText, needle) {
			failures = append(failures, "layout/templates/readme.tmpl: missing "+needle)
		}
	}
	return failures
}

func renderCatalogMarkdown(records []catalogRecord) string {
	var b strings.Builder
	b.WriteString("| Route | Summary | Mutating | Dry-run |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	for _, record := range records {
		mutating := "no"
		if record.Mutating {
			mutating = "yes"
		}
		dryRun := "no"
		if record.SupportsDryRun {
			dryRun = "yes"
		} else if record.SkipDryRunCheck {
			dryRun = "n/a"
		}
		fmt.Fprintf(
			&b,
			"| `%s` | %s | %s | %s |\n",
			record.Path,
			escapeMarkdownCell(record.Summary),
			mutating,
			dryRun,
		)
	}
	return b.String()
}

func escapeMarkdownCell(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}

func runCommandsCheck(cmd *cobra.Command, root *cobra.Command) error {
	report := checkCommandMetadata(root)
	if report.OK {
		return output.OK(cmd, report, "Command metadata is complete")
	}
	return &output.CLIError{
		Code:     output.CodeUsage,
		Message:  "command metadata check failed",
		Hint:     strings.Join(report.Failures, "\n"),
		ExitCode: output.ExitUsage,
		Data:     report,
	}
}

func runCommandsMarkdown(cmd *cobra.Command, root *cobra.Command) error {
	table := renderCatalogMarkdown(flattenCatalog(root))
	if _, err := fmt.Fprint(cmd.OutOrStdout(), table); err != nil {
		return err
	}
	if !strings.HasSuffix(table, "\n") {
		_, err := fmt.Fprintln(cmd.OutOrStdout())
		return err
	}
	return nil
}
