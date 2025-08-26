package main

import (
	"context"
	"log"
	"mbvlabs/andurel/cli"
	"os"
	"time"
)

var (
	version = "v0.1.0"
	date    = time.Now().Format("2006-01-02")
)

func main() {
	ctx := context.Background()

	rootCmd := cli.NewRootCommand(version, date)

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		log.Printf("Error: %v", err)
		os.Exit(1)
	}
}
