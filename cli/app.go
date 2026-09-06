package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

type database struct {
	Port         string
	Host         string
	Name         string
	User         string
	Password     string
	DatabaseKind string
	SslMode      string
}

// GetDatabaseURL returns database URL.
func (d database) GetDatabaseURL() string {
	return fmt.Sprintf("%s://%s:%s@%s:%s/%s?sslmode=%s",
		d.DatabaseKind, d.User, d.Password, d.Host, d.Port,
		d.Name, d.SslMode,
	)
}

func databaseFromEnvironment() (database, error) {
	var errs []error
	required := func(key string) string {
		value, ok := os.LookupEnv(key)
		if !ok || value == "" {
			errs = append(errs, fmt.Errorf("%s is required", key))
		}
		return value
	}
	cfg := database{
		Port:         required("DB_PORT"),
		Host:         required("DB_HOST"),
		Name:         required("DB_NAME"),
		User:         required("DB_USER"),
		Password:     required("DB_PASSWORD"),
		DatabaseKind: required("DB_KIND"),
		SslMode:      required("DB_SSL_MODE"),
	}
	if err := errors.Join(errs...); err != nil {
		return database{}, fmt.Errorf("error parsing environment variables: %w", err)
	}

	return cfg, nil
}

func newConsoleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "console",
		Aliases: []string{"c"},
		Short:   "Open an interactive database console",
		Long: `Open an interactive database console (usql) using the connection
details from .env.

Reads DB_HOST, DB_PORT, DB_NAME, DB_USER, DB_PASSWORD, DB_KIND, and
DB_SSL_MODE from your .env file and connects via usql.`,
		Example: `  andurel console`,
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

			dataCfg, err := databaseFromEnvironment()
			if err != nil {
				return err
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

func newDblabCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "dblab",
		Aliases: []string{"d"},
		Short:   "Open the database UI (dblab)",
		Long: `Open the dblab interactive database UI in the browser.

Uses DB_HOST, DB_PORT, DB_NAME, DB_USER, DB_PASSWORD, DB_KIND, and
DB_SSL_MODE from your .env file.`,
		Example: `  andurel tool dblab`,
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

			dataCfg, err := databaseFromEnvironment()
			if err != nil {
				return err
			}

			dblabPath := filepath.Join(rootDir, "bin", "dblab")
			if _, err := os.Stat(dblabPath); err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf(
						"dblab binary not found at %s\nRun 'andurel tool sync' to download it",
						dblabPath,
					)
				}
				return err
			}

			command := exec.Command(dblabPath, "--url", dataCfg.GetDatabaseURL())
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
  andurel tool mailpit --smtp=0.0.0.0:2525`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}

			binPath := filepath.Join(rootDir, "bin", "mailpit")
			if _, err := os.Stat(binPath); err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf(
						"mailpit binary not found at %s\nRun 'andurel tool sync' to download it",
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
