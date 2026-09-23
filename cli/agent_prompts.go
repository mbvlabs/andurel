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
