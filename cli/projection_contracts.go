package cli

import (
	"fmt"
	"strings"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/spf13/cobra"
)

type projectionSupport struct {
	path    string
	jq      bool
	idsOnly bool
	count   bool
}

const projectionAnnotationPrefix = "andurel.projection."

func configureProjectionContracts(root *cobra.Command) error {
	contracts := []projectionSupport{
		{path: "commands", jq: true},
		{path: "db seed", jq: true},
		{path: "doctor", jq: true},
		{path: "inspect domains", jq: true, idsOnly: true, count: true},
		{path: "inspect handlers", jq: true, idsOnly: true, count: true},
		{path: "inspect jobs", jq: true, idsOnly: true, count: true},
		{path: "inspect migrations", jq: true, idsOnly: true, count: true},
		{path: "inspect project", jq: true},
		{path: "inspect project info", jq: true},
		{path: "inspect routes", jq: true, idsOnly: true, count: true},
		{path: "inspect upgrade-notes", jq: true},
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

	for _, contract := range contracts {
		command, _, err := root.Find(strings.Fields(contract.path))
		if err != nil || command == nil || command == root {
			return fmt.Errorf(
				"configure projection contract for %q: command not found",
				contract.path,
			)
		}
		setProjectionSupport(command, contract.jq, contract.idsOnly, contract.count)
	}
	return nil
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
		}
	}
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
