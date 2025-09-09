package main

import (
	"context"
	"log"
	"github.com/mbvlabs/andurel/cli"
	"os"
	"time"
)

var (
	version string
	date    = time.Now().Format("2006-01-02")
)

func main() {
	ctx := context.Background()

	if version == "" {
		version = "dev"
	}

	rootCmd := cli.NewRootCommand(version, date)

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		log.Printf("Error: %v", err)
		os.Exit(1)
	}
}
