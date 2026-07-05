package cli

import (
	"sort"
	"strings"

	"github.com/mbvlabs/andurel/cli/output"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	agentNotesAnnotation    = "agent.notes"
	agentCategoryAnnotation = "agent.category"
)

type commandDiscovery struct {
	Name           string             `json:"name"`
	Path           string             `json:"path"`
	Aliases        []string           `json:"aliases,omitempty"`
	Short          string             `json:"short,omitempty"`
	Long           string             `json:"long,omitempty"`
	Usage          string             `json:"usage"`
	Examples       string             `json:"examples,omitempty"`
	LocalFlags     []flagDiscovery    `json:"local_flags,omitempty"`
	InheritedFlags []flagDiscovery    `json:"inherited_flags,omitempty"`
	Subcommands    []commandSummary   `json:"subcommands,omitempty"`
	AgentNotes     string             `json:"agent_notes,omitempty"`
	Category       string             `json:"category,omitempty"`
	Commands       []commandDiscovery `json:"commands,omitempty"`
}

type commandSummary struct {
	Name     string   `json:"name"`
	Path     string   `json:"path"`
	Aliases  []string `json:"aliases,omitempty"`
	Short    string   `json:"short,omitempty"`
	Category string   `json:"category,omitempty"`
}

type flagDiscovery struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand,omitempty"`
	Usage     string `json:"usage,omitempty"`
	Default   string `json:"default,omitempty"`
	Type      string `json:"type,omitempty"`
}

func newCommandsCommand(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "commands",
		Short: "Show structured command discovery data",
		Long:  "Show the Andurel command tree, flags, descriptions, examples, and agent metadata.",
		Example: `  andurel commands --json
  andurel commands --agent`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			target := root
			if target == nil {
				target = cmd.Root()
			}
			return output.OK(cmd, discoverCommandTree(target), "Discovered Andurel commands")
		},
	}
	setAgentMetadata(cmd, "discovery", "Returns structured command metadata for agent planning.")
	return cmd
}

func renderStructuredHelpIfNeeded(cmd *cobra.Command) bool {
	opts, err := output.ParseOptions(cmd)
	if err != nil {
		_ = output.RenderError(cmd, err)
		return true
	}
	if opts.Mode != output.ModeAgent && opts.Mode != output.ModeJSON {
		return false
	}

	_ = output.OK(cmd, discoverCommand(cmd), "Command help for "+cmd.CommandPath())
	return true
}

func discoverCommandTree(root *cobra.Command) commandDiscovery {
	discovery := discoverCommand(root)
	for _, cmd := range availableSubcommands(root) {
		discovery.Commands = append(discovery.Commands, discoverCommandTree(cmd))
	}
	return discovery
}

func discoverCommand(cmd *cobra.Command) commandDiscovery {
	discovery := commandDiscovery{
		Name:           cmd.Name(),
		Path:           cmd.CommandPath(),
		Aliases:        append([]string(nil), cmd.Aliases...),
		Short:          strings.TrimSpace(cmd.Short),
		Long:           strings.TrimSpace(cmd.Long),
		Usage:          cmd.UseLine(),
		Examples:       strings.TrimSpace(cmd.Example),
		LocalFlags:     discoverFlags(cmd.NonInheritedFlags()),
		InheritedFlags: discoverFlags(cmd.InheritedFlags()),
		Subcommands:    discoverSubcommands(cmd),
	}

	if cmd.Annotations != nil {
		discovery.AgentNotes = cmd.Annotations[agentNotesAnnotation]
		discovery.Category = cmd.Annotations[agentCategoryAnnotation]
	}

	return discovery
}

func discoverSubcommands(cmd *cobra.Command) []commandSummary {
	commands := availableSubcommands(cmd)
	summaries := make([]commandSummary, 0, len(commands))
	for _, sub := range commands {
		summary := commandSummary{
			Name:    sub.Name(),
			Path:    sub.CommandPath(),
			Aliases: append([]string(nil), sub.Aliases...),
			Short:   strings.TrimSpace(sub.Short),
		}
		if sub.Annotations != nil {
			summary.Category = sub.Annotations[agentCategoryAnnotation]
		}
		summaries = append(summaries, summary)
	}
	return summaries
}

func availableSubcommands(cmd *cobra.Command) []*cobra.Command {
	commands := make([]*cobra.Command, 0)
	for _, sub := range cmd.Commands() {
		if !sub.IsAvailableCommand() || sub.Hidden {
			continue
		}
		commands = append(commands, sub)
	}
	return commands
}

func discoverFlags(flags *pflag.FlagSet) []flagDiscovery {
	if flags == nil {
		return nil
	}

	discovered := make([]flagDiscovery, 0)
	flags.VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		discovered = append(discovered, flagDiscovery{
			Name:      flag.Name,
			Shorthand: flag.Shorthand,
			Usage:     strings.TrimSpace(flag.Usage),
			Default:   flag.DefValue,
			Type:      flag.Value.Type(),
		})
	})
	sort.SliceStable(discovered, func(i, j int) bool {
		return discovered[i].Name < discovered[j].Name
	})
	return discovered
}

func setAgentMetadata(cmd *cobra.Command, category, notes string) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[agentCategoryAnnotation] = category
	cmd.Annotations[agentNotesAnnotation] = notes
}
