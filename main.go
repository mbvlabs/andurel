package main

import (
	"context"
	"os"
	"runtime/debug"
	"time"

	"github.com/mbvlabs/andurel/cli"
)

var (
	version string
	date    = time.Now().Format("2006-01-02")
)

func getVersion() string {
	if version != "" {
		return version
	}

	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
	}

	return "dev"
}

func main() {
	ctx := context.Background()

	rootCmd := cli.NewRootCommand(getVersion(), date)

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
