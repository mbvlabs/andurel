package main

import (
	"context"
	"log"
	"os"
	"runtime/debug"
	"time"

	"github.com/mbvlabs/andurel/cli"
)

var date = time.Now().Format("2006-01-02")

func getVersion() string {
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		if buildInfo.Main.Version != "(devel)" && buildInfo.Main.Version != "" {
			return buildInfo.Main.Version
		}
	}
	return "dev"
}

func main() {
	ctx := context.Background()

	version := getVersion()
	rootCmd := cli.NewRootCommand(version, date)

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		log.Printf("Error: %v", err)
		os.Exit(1)
	}
}
