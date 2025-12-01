package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

func newAppCommand() *cobra.Command {
	appCmd := &cobra.Command{
		Use:     "app",
		Aliases: []string{"a"},
		Short:   "App related commands",
		Long:    "Commands related to running and managing the application.",
	}

	appCmd.AddCommand(newConsoleCommand())
	appCmd.AddCommand(newMailpitCommand())

	return appCmd
}

type database struct {
	Port         string `env:"DB_PORT"`
	Host         string `env:"DB_HOST"`
	Name         string `env:"DB_NAME"`
	User         string `env:"DB_USER"`
	Password     string `env:"DB_PASSWORD"`
	DatabaseKind string `env:"DB_KIND"`
	SslMode      string `env:"DB_SSL_MODE"`
}

func (d database) GetDatabaseURL() string {
	return fmt.Sprintf("%s://%s:%s@%s:%s/%s?sslmode=%s",
		d.DatabaseKind, d.User, d.Password, d.Host, d.Port,
		d.Name, d.SslMode,
	)
}

func newConsoleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "console",
		Aliases: []string{"c"},
		Short:   "Runs an interactive database console",
		Args:    cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}

			envPath := filepath.Join(rootDir, ".env")
			if _, err := os.Stat(envPath); err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf(
						".env file not found at %s\nCreate one to set your environment variables",
						envPath,
					)
				}
				return err
			}

			if err := godotenv.Load(envPath); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not load .env file: %v\n", err)
			}

			dataCfg := database{}

			if err := env.ParseWithOptions(&dataCfg, env.Options{
				RequiredIfNoDef: true,
			}); err != nil {
				return fmt.Errorf("error parsing environment variables: %w", err)
			}

			usqlPath := filepath.Join(rootDir, "bin", "usql")
			if _, err := os.Stat(usqlPath); err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf(
						"usql binary not found at %s\nRun 'andurel tool sync' to download it",
						usqlPath,
					)
				}
				return err
			}

			command := exec.Command(usqlPath, dataCfg.GetDatabaseURL())
			command.Stdout = os.Stdout
			command.Stderr = os.Stderr
			command.Stdin = os.Stdin
			command.Dir = rootDir

			return command.Run()
		},
	}

	return cmd
}

func newMailpitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "mailpit",
		Aliases: []string{"m"},
		Short:   "Runs the Mailpit email server",
		Long: `Runs the Mailpit email server with default configuration.

Default bindings:
  - SMTP: 0.0.0.0:1025
  - HTTP: 0.0.0.0:8025

Override defaults by passing flags, e.g.:
  andurel app mailpit --smtp=0.0.0.0:2525`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}

			binPath := filepath.Join(rootDir, "bin", "mailpit")
			if _, err := os.Stat(binPath); err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf(
						"mailpit binary not found at %s\nRun 'andurel sync' to download it",
						binPath,
					)
				}
				return err
			}

			defaultArgs := []string{
				"--smtp=0.0.0.0:1025",
				"--listen=0.0.0.0:8025",
			}

			finalArgs := append(defaultArgs, args...)

			command := exec.Command(binPath, finalArgs...)
			command.Stdout = os.Stdout
			command.Stderr = os.Stderr
			command.Stdin = os.Stdin
			command.Dir = rootDir

			return command.Run()
		},
	}

	return cmd
}
