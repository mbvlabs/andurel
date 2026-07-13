// Andurel scaffolds and manages full-stack Go web applications.
package main

import (
	"context"
	"os"
	"runtime/debug"
	"time"

	"github.com/mbvlabs/andurel/cli"
	"github.com/mbvlabs/andurel/cli/output"
	"github.com/spf13/cobra"
)

var (
	version string
	date    = time.Now().Format("2006-01-02")
)

var readBuildInfo = debug.ReadBuildInfo

func getVersion() string {
	if version != "" {
		return version
	}

	if info, ok := readBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
	}

	return "v1.3.0"
}

func execute(ctx context.Context, rootCmd *cobra.Command) int {
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		_ = output.RenderError(rootCmd, err)
		return output.ExitCode(err)
	}
	return 0
}

func main() {
	ctx := context.Background()
	rootCmd := cli.NewRootCommand(getVersion(), date)
	if exitCode := execute(ctx, rootCmd); exitCode != 0 {
		os.Exit(exitCode)
	}
}
