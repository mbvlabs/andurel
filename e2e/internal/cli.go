package internal

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func RunCLI(t *testing.T, binary, workDir string, env []string, args ...string) error {
	t.Helper()

	return RunCommand(t, binary, workDir, env, args...)
}

func RunCommand(t *testing.T, cmdName, workDir string, env []string, args ...string) error {
	t.Helper()

	return runCommandInternal(t, cmdName, workDir, env, true, args...)
}

// RunCommandExpectError runs a command that is expected to fail, suppressing failure logs.
func RunCommandExpectError(t *testing.T, cmdName, workDir string, env []string, args ...string) error {
	t.Helper()

	return runCommandInternal(t, cmdName, workDir, env, false, args...)
}

func runCommandInternal(t *testing.T, cmdName, workDir string, env []string, logOnError bool, args ...string) error {
	t.Helper()

	cmd := exec.Command(cmdName, args...)
	cmd.Dir = workDir

	cmd.Env = os.Environ()
	if env != nil {
		cmd.Env = append(cmd.Env, env...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if logOnError {
			t.Logf("Command failed: %s %s", cmdName, strings.Join(args, " "))
			t.Logf("Working directory: %s", workDir)
			t.Logf("Stdout:\n%s", stdout.String())
			t.Logf("Stderr:\n%s", stderr.String())
		}
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}

func RunCommandOutput(t *testing.T, cmdName, workDir string, env []string, args ...string) (string, error) {
	t.Helper()

	cmd := exec.Command(cmdName, args...)
	cmd.Dir = workDir

	cmd.Env = os.Environ()
	if env != nil {
		cmd.Env = append(cmd.Env, env...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Command failed: %s %s", cmdName, strings.Join(args, " "))
		t.Logf("Working directory: %s", workDir)
		t.Logf("Output:\n%s", string(output))
		return string(output), fmt.Errorf("command failed: %w", err)
	}

	return string(output), nil
}
