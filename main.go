package main

import (
	"context"
	"os"
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
	return "dev"
}

func main() {
	ctx := context.Background()

	version := getVersion()
	rootCmd := cli.NewRootCommand(version, date)

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
