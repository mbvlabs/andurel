package main

import (
	"bytes"
	"context"
	"errors"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/cli/output"
	"github.com/spf13/cobra"
)

func TestGetVersionUsesExplicitVersionAndFallback(t *testing.T) {
	original := version
	originalReadBuildInfo := readBuildInfo
	t.Cleanup(func() {
		version = original
		readBuildInfo = originalReadBuildInfo
	})

	version = "v9.9.9"
	if got := getVersion(); got != "v9.9.9" {
		t.Fatalf("getVersion explicit = %q", got)
	}

	version = ""
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "v2.4.6"}}, true
	}
	if got := getVersion(); got != "v2.4.6" {
		t.Fatalf("getVersion build info = %q", got)
	}

	for _, buildVersion := range []string{"", "(devel)"} {
		readBuildInfo = func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Main: debug.Module{Version: buildVersion}}, true
		}
		if got := getVersion(); got != "v1.3.0" {
			t.Fatalf("getVersion fallback for %q = %q", buildVersion, got)
		}
	}

	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }
	if got := getVersion(); got != "v1.3.0" {
		t.Fatalf("getVersion unavailable build info = %q", got)
	}
}

func TestExecute(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cmd := &cobra.Command{Run: func(*cobra.Command, []string) {}}
		if got := execute(context.Background(), cmd); got != 0 {
			t.Fatalf("execute exit code = %d, want 0", got)
		}
	})

	t.Run("error", func(t *testing.T) {
		expectedErr := output.NewError(output.CodeConfigError, "invalid config", output.ExitConfig, "repair it")
		cmd := &cobra.Command{
			SilenceErrors: true,
			SilenceUsage:  true,
			RunE: func(*cobra.Command, []string) error {
				return expectedErr
			},
		}
		var stderr bytes.Buffer
		cmd.SetErr(&stderr)

		if got := execute(context.Background(), cmd); got != output.ExitConfig {
			t.Fatalf("execute exit code = %d, want %d", got, output.ExitConfig)
		}
		for _, want := range []string{"invalid config", "repair it"} {
			if !strings.Contains(stderr.String(), want) {
				t.Fatalf("stderr missing %q: %s", want, stderr.String())
			}
		}
	})

	t.Run("generic error", func(t *testing.T) {
		cmd := &cobra.Command{
			SilenceErrors: true,
			SilenceUsage:  true,
			RunE: func(*cobra.Command, []string) error {
				return errors.New("boom")
			},
		}
		if got := execute(context.Background(), cmd); got != output.ExitUsage {
			t.Fatalf("execute exit code = %d, want %d", got, output.ExitUsage)
		}
	})
}
