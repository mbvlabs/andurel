package cli

import (
	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/spf13/cobra"
)

func usesStructuredOutput(cmd *cobra.Command) (bool, error) {
	if cmd == nil {
		return false, nil
	}
	opts, err := output.ParseOptions(cmd)
	if err != nil {
		return false, err
	}
	return output.UsesStructuredOutput(opts), nil
}

func requireFlagInStructuredMode(cmd *cobra.Command, provided bool, flag, action string) error {
	structured, err := usesStructuredOutput(cmd)
	if err != nil {
		return err
	}
	if !structured || provided {
		return nil
	}
	return output.NewError(
		output.CodeUsage,
		action+" cannot prompt in --json or --agent mode",
		output.ExitUsage,
		"Pass "+flag+" to continue without a TTY prompt.",
	)
}

func confirmDestructiveAction(
	cmd *cobra.Command,
	force bool,
	action string,
	databaseName string,
) error {
	if err := requireFlagInStructuredMode(cmd, force, "--force", "destructive database "+action); err != nil {
		return err
	}
	if force {
		return nil
	}
	confirmed, err := confirmDestructive(action, databaseName)
	if err != nil {
		return err
	}
	if !confirmed {
		return errDatabaseOperationAborted
	}
	return nil
}

// rejectStructuredOutput fails fast for interactive or passthrough commands
// that stream an external process to the TTY and cannot honor the shared
// --json/--agent/--md envelope. Agents must not guess at unstructured output.
func rejectStructuredOutput(cmd *cobra.Command, action string) error {
	structured, err := usesStructuredOutput(cmd)
	if err != nil {
		return err
	}
	if !structured {
		return nil
	}
	return output.NewError(
		output.CodeUsage,
		action+" does not support --json, --agent, or --md",
		output.ExitUsage,
		"Run without a structured output flag, or use andurel commands --json to find a machine-readable alternative.",
	)
}
