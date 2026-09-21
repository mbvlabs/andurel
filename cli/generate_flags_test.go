package cli

import (
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/cli/output"
	"github.com/mbvlabs/andurel/layout"
)

func TestNoArgGeneratorCommandsShowHelpWithoutCallingGenerators(t *testing.T) {
	tests := [][]string{
		{"generate", "model"},
		{"generate", "controller"},
		{"generate", "scaffold"},
		{"generate", "job"},
		{"generate", "email"},
	}

	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			resetCLITestSeams(t)
			fake := installFakeGenerator(t)

			result := executeCLITest(t, args...)
			if result.err != nil {
				t.Fatalf("expected help without error, got %v", result.err)
			}
			if len(fake.modelCalls) != 0 || len(fake.modelWithPKCalls) != 0 ||
				len(fake.scaffoldCalls) != 0 ||
				len(fake.controllerCalls) != 0 {
				t.Fatalf("expected no generator calls, got %#v", fake)
			}
		})
	}
}

func TestGenerateCommandsRejectTooManyArgs(t *testing.T) {
	tests := []struct {
		args    []string
		message string
	}{
		{
			args:    []string{"generate", "model", "Post", "Extra"},
			message: "model takes exactly 1 argument",
		},
		{
			args:    []string{"generate", "scaffold", "Post", "Extra"},
			message: "scaffold takes exactly 1 argument",
		},
		{
			args:    []string{"generate", "job", "SendEmail", "Extra"},
			message: "job takes exactly 1 argument",
		},
		{
			args:    []string{"generate", "email", "WelcomeEmail", "Extra"},
			message: "email takes exactly 1 argument",
		},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			result := runCLITest(t, tt.args...)
			if result.err == nil {
				t.Fatalf("expected error")
			}
			if !strings.Contains(result.err.Error(), tt.message) {
				t.Fatalf("expected error containing %q, got %v", tt.message, result.err)
			}
		})
	}
}

func TestInertiaGenerationRequiresConfiguredProjectAndRejectsAPI(t *testing.T) {
	root := t.TempDir()
	writeCLITestFile(t, root, "go.mod", "module example.com/app\n")
	lock := layout.NewAndurelLock("test")
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	resetCLITestSeams(t)
	findGoModRoot = func() (string, error) { return root, nil }

	for _, arguments := range [][]string{
		{"Product", "--inertia"},
		{"Product", "--inertia", "--api"},
	} {
		cmd := newGenerateControllerCommand()
		cmd.SetArgs(arguments)
		err := cmd.Execute()
		if err == nil {
			t.Fatalf("controller %v should fail", arguments)
		}
		envelope := output.Fail(err)
		if len(arguments) == 2 && envelope.Code != output.CodeInvalidInertiaAdapter {
			t.Fatalf("controller %v error = %#v", arguments, envelope)
		}
		if len(arguments) == 3 && envelope.Code != output.CodeUsage {
			t.Fatalf("controller %v error = %#v", arguments, envelope)
		}
	}
}

func TestGenerateModelRejectsInvalidMode(t *testing.T) {
	result := runCLITest(t, "generate", "model", "AuditLog", "--mode", "immutable")
	if result.err == nil || !strings.Contains(result.err.Error(), "invalid model mode") {
		t.Fatalf("expected invalid model mode error, got %v", result.err)
	}
}
