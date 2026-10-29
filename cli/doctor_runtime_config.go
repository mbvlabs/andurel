package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// The generated project owns the runtime config contract: internal/configcheck
// loads the same config constructor cmd/app uses, then probes the database
// read-only. doctor runs it instead of re-implementing the constructor's
// required values, so a config change in the project can never drift from the
// check.
const (
	configCheckPath = "internal/configcheck/main.go"

	// configCheckTimeout bounds the whole probe, including a cold go build.
	// A check that cannot finish is reported as a failure: runtime readiness is
	// then unverified, which must not pass. Only a project without the generated
	// helper and a missing Go toolchain are reported as blocked/skipped.
	configCheckTimeout = 2 * time.Minute
)

// configCheckReport is the JSON document internal/configcheck prints.
type configCheckReport struct {
	OK       bool                `json:"ok"`
	Config   configCheckSection  `json:"config"`
	Database *configCheckSection `json:"database,omitempty"`
}

// configCheckSection is one validated area: "valid"/"invalid" for config,
// "reachable"/"unreachable"/"skipped" for the database probe.
type configCheckSection struct {
	Status string   `json:"status"`
	Reason string   `json:"reason,omitempty"`
	Errors []string `json:"errors,omitempty"`
}

// runProjectConfigCheckFunc is a seam so doctor tests can drive each config
// and database outcome without a Go toolchain or a database.
var runProjectConfigCheckFunc = runProjectConfigCheck

func runProjectConfigCheck(rootDir string) (configCheckReport, error) {
	ctx, cancel := context.WithTimeout(context.Background(), configCheckTimeout)
	defer cancel()

	stdout, stderr, err := runCommandInDir(ctx, rootDir, "go", "run", "./internal/configcheck")
	if err != nil {
		return configCheckReport{}, configCheckRunError(err, stderr)
	}

	var report configCheckReport
	if err := json.Unmarshal(stdout, &report); err != nil {
		return configCheckReport{}, fmt.Errorf("decode config check output: %w", err)
	}

	return report, nil
}

// configCheckRunError keeps the first stderr lines so a build or toolchain
// failure is actionable without dumping a whole compile log.
func configCheckRunError(err error, stderr []byte) error {
	lines := make([]string, 0, 5)
	for line := range strings.SplitSeq(strings.TrimSpace(string(stderr)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
		if len(lines) == 5 {
			break
		}
	}
	if len(lines) == 0 {
		return err
	}

	return fmt.Errorf("%w: %s", err, strings.Join(lines, "; "))
}

// runCommandInDir runs a command in dir and kills its whole process group when
// ctx ends, so a bounded check cannot leave a child behind.
func runCommandInDir(
	ctx context.Context,
	dir, name string,
	args ...string,
) (stdout, stderr []byte, err error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		<-done
		return stdoutBuf.Bytes(), stderrBuf.Bytes(), ctx.Err()
	case waitErr := <-done:
		return stdoutBuf.Bytes(), stderrBuf.Bytes(), waitErr
	}
}

// checkRuntimeConfiguration validates the project's runtime configuration and
// its database connection. Missing or malformed required values and an
// unreachable database are blocking: cmd/app cannot start without them.
func checkRuntimeConfiguration(rootDir string) checkResult {
	const name = "Runtime config"

	if _, err := os.Stat(filepath.Join(rootDir, filepath.FromSlash(configCheckPath))); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return checkResult{
				name:    name,
				status:  statusSkip,
				message: "cannot read the generated config check",
				details: []string{err.Error()},
				hint:    "Check filesystem permissions on " + configCheckPath + ".",
			}
		}

		return checkResult{
			name:    name,
			status:  statusSkip,
			message: "project has no generated config check",
			hint: "Create " + configCheckPath + " from a current andurel new scaffold " +
				"(andurel upgrade does not manage internal/ files) to validate runtime configuration.",
		}
	}

	report, err := runProjectConfigCheckFunc(rootDir)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return checkResult{
				name:    name,
				status:  statusSkip,
				message: "Go toolchain is not available",
				details: []string{err.Error()},
				hint: "Install Go (https://go.dev/dl/) and run andurel doctor again; " +
					"runtime configuration cannot be verified without it.",
			}
		}

		return checkResult{
			name:    name,
			status:  statusFail,
			message: "config check did not run",
			details: []string{err.Error()},
			hint: "Fix the generated " + configCheckPath + " build error and run " +
				"go run ./internal/configcheck to see the full output; runtime " +
				"configuration stays unverified until the check runs.",
		}
	}

	if report.Config.Status != "valid" {
		message := "required runtime configuration is missing or invalid"
		if len(report.Config.Errors) == 0 && report.Config.Reason != "" {
			message = report.Config.Reason
		}

		return checkResult{
			name:    name,
			status:  statusFail,
			message: message,
			details: append([]string(nil), report.Config.Errors...),
			hint: "Copy .env.example to .env and set the reported keys, or export them; " +
				"run andurel doctor again afterwards.",
		}
	}

	if database := report.Database; database != nil && database.Status == "unreachable" {
		details := make([]string, 0, 1)
		if database.Reason != "" {
			details = append(details, database.Reason)
		}

		return checkResult{
			name:    name,
			status:  statusFail,
			message: "database is unreachable",
			details: details,
			hint: "Start the database from DB_HOST/DB_PORT/DB_NAME, create it if needed, " +
				"then run andurel db migrate up.",
		}
	}

	if database := report.Database; database != nil && database.Status == "skipped" {
		return checkResult{
			name:    name,
			status:  statusWarn,
			message: "configuration is valid; database probe skipped",
			details: []string{database.Reason},
		}
	}

	return checkResult{
		name:    name,
		status:  statusPass,
		message: "configuration is valid and the database answers",
	}
}
