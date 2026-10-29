package cli

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/spf13/cobra"
)

// Projection flags select a field or collection from a command's data. They
// are separate capabilities from structured --json/--agent/--md output: a
// command can emit a JSON envelope without supporting --jq, --ids-only, or
// --count.
type projectionSupport struct {
	path    string
	jq      bool
	idsOnly bool
	count   bool
}

const (
	projectionAnnotationPrefix = "andurel.projection."
	capabilityAnnotationJSON   = "andurel.capability.json"
)

// projectionContracts is the single registration point for projection flags.
// Every entry must name a public command that supports structured output; a
// contradiction fails command construction instead of shipping a broken
// capability claim.
var projectionContracts = []projectionSupport{
	{path: "commands", jq: true},
	{path: "db seed", jq: true},
	{path: "doctor", jq: true},
	{path: "inspect config", jq: true},
	{path: "inspect domains", jq: true, idsOnly: true, count: true},
	{path: "inspect handlers", jq: true, idsOnly: true, count: true},
	{path: "inspect jobs", jq: true, idsOnly: true, count: true},
	{path: "inspect migrations", jq: true, idsOnly: true, count: true},
	{path: "inspect pages", jq: true},
	{path: "inspect process", jq: true},
	{path: "inspect project", jq: true},
	{path: "inspect project info", jq: true},
	{path: "inspect routes", jq: true, idsOnly: true, count: true},
	{path: "inspect upgrade-notes", jq: true},
	{path: "inspect urls", jq: true},
	{path: "new", jq: true},
	{path: "packages", jq: true, idsOnly: true, count: true},
	{path: "packages list", jq: true, idsOnly: true, count: true},
	{path: "packages update", jq: true},
	{path: "patterns", jq: true, idsOnly: true, count: true},
	{path: "patterns list", jq: true, idsOnly: true, count: true},
	{path: "patterns show", jq: true},
	{path: "skill install", jq: true},
	{path: "skill show", jq: true},
	{path: "sync email", jq: true},
	{path: "sync payloads", jq: true},
	{path: "sync queries", jq: true},
	{path: "sync routes", jq: true},
	{path: "tool", jq: true, idsOnly: true, count: true},
	{path: "tool list", jq: true, idsOnly: true, count: true},
	{path: "upgrade", jq: true},
	{path: "verify", jq: true},
}

// structuredOutputUnsupportedCommands stream an external process to the TTY
// or are interactive, so they reject --json/--agent/--md. Group commands are
// not listed: --json on a group renders the generic command descriptor.
var structuredOutputUnsupportedCommands = []string{
	"andurel console",
	"andurel db console",
	"andurel db create",
	"andurel db drop",
	"andurel db migrate down",
	"andurel db migrate down-to",
	"andurel db migrate fix",
	"andurel db migrate reset",
	"andurel db migrate status",
	"andurel db migrate up",
	"andurel db migrate up-to",
	"andurel db nuke",
	"andurel db rebuild",
	"andurel fmt",
	"andurel generate migration",
	"andurel run",
	"andurel tool dblab",
	"andurel tool mailpit",
	"andurel tool set-version",
}

func configureCommandCapabilities(root *cobra.Command) error {
	for _, command := range walkPublicCommands(root) {
		supportsJSON := !structuredOutputUnsupported(command.CommandPath())
		setCommandCapability(command, supportsJSON)
	}
	if err := applyProjectionContracts(root, projectionContracts); err != nil {
		return err
	}
	for _, command := range walkPublicCommands(root) {
		if !projectionRegistered(command) {
			continue
		}
		if command.Annotations[capabilityAnnotationJSON] != "true" {
			return fmt.Errorf(
				"command %q registers a projection but not structured output support",
				command.CommandPath(),
			)
		}
	}
	return nil
}

func applyProjectionContracts(root *cobra.Command, contracts []projectionSupport) error {
	for _, contract := range contracts {
		command, _, err := root.Find(strings.Fields(contract.path))
		if err != nil || command == nil || command == root {
			return fmt.Errorf(
				"configure projection contract for %q: command not found",
				contract.path,
			)
		}
		if contract.jq || contract.idsOnly || contract.count {
			if command.Annotations[capabilityAnnotationJSON] != "true" {
				return fmt.Errorf(
					"configure projection contract for %q: command does not support structured output",
					contract.path,
				)
			}
		}
		setProjectionSupport(command, contract.jq, contract.idsOnly, contract.count)
	}
	return nil
}

func setCommandCapability(cmd *cobra.Command, supportsJSON bool) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[capabilityAnnotationJSON] = strconv.FormatBool(supportsJSON)
}

func setProjectionSupport(cmd *cobra.Command, jq, idsOnly, count bool) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	for name, supported := range map[string]bool{
		"jq":       jq,
		"ids-only": idsOnly,
		"count":    count,
	} {
		if supported {
			cmd.Annotations[projectionAnnotationPrefix+name] = "true"
			continue
		}
		delete(cmd.Annotations, projectionAnnotationPrefix+name)
	}
}

func projectionRegistered(cmd *cobra.Command) bool {
	return cmd.Annotations[projectionAnnotationPrefix+"jq"] == "true" ||
		cmd.Annotations[projectionAnnotationPrefix+"ids-only"] == "true" ||
		cmd.Annotations[projectionAnnotationPrefix+"count"] == "true"
}

func structuredOutputUnsupported(path string) bool {
	return slices.Contains(structuredOutputUnsupportedCommands, path)
}

func validateProjectionFlags(cmd *cobra.Command, _ []string) error {
	opts, err := output.ParseOptions(cmd)
	if err != nil {
		return err
	}

	selected := ""
	switch {
	case opts.JQ != "":
		selected = "jq"
	case opts.IDsOnly:
		selected = "ids-only"
	case opts.Count:
		selected = "count"
	default:
		return nil
	}

	if cmd.Annotations[projectionAnnotationPrefix+selected] == "true" {
		return nil
	}
	return output.NewError(
		output.CodeUsage,
		fmt.Sprintf("--%s is not supported by %s", selected, cmd.CommandPath()),
		output.ExitUsage,
		"Run the command without that projection flag or choose a command that documents projection support.",
	)
}
