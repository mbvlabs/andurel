package cli

import (
	"fmt"
	"strings"

	"github.com/mbvlabs/andurel/cli/output"
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
		{path: "config init", jq: true},
		{path: "config set", jq: true},
		{path: "config show", jq: true},
		{path: "config unset", jq: true},
		{path: "controllers", jq: true, idsOnly: true, count: true},
		{path: "database seed", jq: true},
		{path: "doctor", jq: true},
		{path: "extension", jq: true, idsOnly: true, count: true},
		{path: "extension add", jq: true},
		{path: "extension list", jq: true, idsOnly: true, count: true},
		{path: "generate controller", jq: true},
		{path: "generate email", jq: true},
		{path: "generate factories", jq: true},
		{path: "generate factory", jq: true},
		{path: "generate job", jq: true},
		{path: "generate model", jq: true},
		{path: "generate routes", jq: true},
		{path: "generate scaffold", jq: true},
		{path: "jobs", jq: true, idsOnly: true, count: true},
		{path: "migrations", jq: true, idsOnly: true, count: true},
		{path: "models", jq: true, idsOnly: true, count: true},
		{path: "new", jq: true},
		{path: "project", jq: true},
		{path: "project info", jq: true},
		{path: "routes", jq: true, idsOnly: true, count: true},
		{path: "skill install", jq: true},
		{path: "skill show", jq: true},
		{path: "tool", jq: true, idsOnly: true, count: true},
		{path: "tool list", jq: true, idsOnly: true, count: true},
		{path: "upgrade", jq: true},
		{path: "views", jq: true, idsOnly: true, count: true},
	}

	for _, contract := range contracts {
		command, _, err := root.Find(strings.Fields(contract.path))
		if err != nil || command == nil || command == root {
			return fmt.Errorf("configure projection contract for %q: command not found", contract.path)
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
