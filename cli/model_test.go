package cli

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/generator"
)

func changedModelUpdate() *generator.UpdateModelResult {
	return &generator.UpdateModelResult{
		OldStruct:         "type WidgetEntity struct {\n\tName string\n}\n",
		NewStruct:         "type WidgetEntity struct {\n\tName string\n\tCount int64\n}\n",
		ModelPath:         "models/widget.go",
		HasChanges:        true,
		OldFactoryContent: "package factories\n\nvar Count = 1\n",
		NewFactoryContent: "package factories\n\nvar Count = 2\n",
		FactoryPath:       "models/factories/widget.go",
		FactoryHasChanges: true,
	}
}

func TestRunModelUpdateAppliesModelAndFactory(t *testing.T) {
	resetCLITestSeams(t)
	fake := installFakeGenerator(t)
	fake.modelUpdate = changedModelUpdate()
	capture := captureProcessOutput(t, &os.Stdout)

	err := runModelUpdate("Widget", true, false)
	out := capture()
	if err != nil {
		t.Fatalf("runModelUpdate: %v", err)
	}
	if len(fake.modelUpdateCalls) != 1 || fake.modelUpdateCalls[0] != "Widget" {
		t.Fatalf("update calls = %#v", fake.modelUpdateCalls)
	}
	if len(fake.modelApplyCalls) != 1 || fake.modelApplyCalls[0] != fake.modelUpdate {
		t.Fatalf("apply calls = %#v", fake.modelApplyCalls)
	}
	for _, want := range []string{
		"Changes to models/widget.go",
		"Changes to models/factories/widget.go",
		"Updated models/widget.go",
		"Updated models/factories/widget.go",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRunModelUpdateSkipsFactory(t *testing.T) {
	resetCLITestSeams(t)
	fake := installFakeGenerator(t)
	fake.modelUpdate = &generator.UpdateModelResult{
		FactoryPath:       "models/factories/widget.go",
		OldFactoryContent: "old",
		NewFactoryContent: "new",
		FactoryHasChanges: true,
	}
	capture := captureProcessOutput(t, &os.Stdout)

	err := runModelUpdate("Widget", true, true)
	out := capture()
	if err != nil {
		t.Fatalf("runModelUpdate: %v", err)
	}
	if len(fake.modelApplyCalls) != 0 {
		t.Fatalf("unexpected apply calls: %#v", fake.modelApplyCalls)
	}
	if fake.modelUpdate.FactoryPath != "" || fake.modelUpdate.FactoryHasChanges {
		t.Fatalf("factory result was not cleared: %#v", fake.modelUpdate)
	}
	if !strings.Contains(out, "No changes") {
		t.Fatalf("missing no changes output: %s", out)
	}
}

func TestRunModelUpdatePrompts(t *testing.T) {
	t.Run("declined", func(t *testing.T) {
		resetCLITestSeams(t)
		fake := installFakeGenerator(t)
		fake.modelUpdate = changedModelUpdate()
		originalStdin := os.Stdin
		os.Stdin = tempInputFile(t, "n\n")
		t.Cleanup(func() { os.Stdin = originalStdin })
		capture := captureProcessOutput(t, &os.Stdout)

		err := runModelUpdate("Widget", false, false)
		out := capture()
		if err != nil {
			t.Fatalf("runModelUpdate: %v", err)
		}
		if len(fake.modelApplyCalls) != 0 {
			t.Fatalf("unexpected apply calls: %#v", fake.modelApplyCalls)
		}
		if !strings.Contains(out, "Aborted") {
			t.Fatalf("missing aborted output: %s", out)
		}
	})

	t.Run("confirmed", func(t *testing.T) {
		resetCLITestSeams(t)
		fake := installFakeGenerator(t)
		fake.modelUpdate = changedModelUpdate()
		originalStdin := os.Stdin
		os.Stdin = tempInputFile(t, "yes\n")
		t.Cleanup(func() { os.Stdin = originalStdin })
		capture := captureProcessOutput(t, &os.Stdout)

		err := runModelUpdate("Widget", false, false)
		_ = capture()
		if err != nil {
			t.Fatalf("runModelUpdate: %v", err)
		}
		if len(fake.modelApplyCalls) != 1 {
			t.Fatalf("apply calls = %#v", fake.modelApplyCalls)
		}
	})

	t.Run("input error", func(t *testing.T) {
		resetCLITestSeams(t)
		fake := installFakeGenerator(t)
		fake.modelUpdate = changedModelUpdate()
		input := tempInputFile(t, "")
		if err := input.Close(); err != nil {
			t.Fatalf("close input: %v", err)
		}
		originalStdin := os.Stdin
		os.Stdin = input
		t.Cleanup(func() { os.Stdin = originalStdin })
		capture := captureProcessOutput(t, &os.Stdout)

		err := runModelUpdate("Widget", false, false)
		_ = capture()
		if err == nil {
			t.Fatal("expected prompt input error")
		}
		if len(fake.modelApplyCalls) != 0 {
			t.Fatalf("unexpected apply calls: %#v", fake.modelApplyCalls)
		}
	})
}

func TestRunModelUpdateErrors(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*fakeGenerator)
		want      error
	}{
		{name: "generator", configure: func(fake *fakeGenerator) { fake.err = errGeneratorFactory }, want: errGeneratorFactory},
		{name: "update", configure: func(fake *fakeGenerator) { fake.modelUpdateErr = errors.New("update failed") }},
		{name: "apply", configure: func(fake *fakeGenerator) {
			fake.modelUpdate = changedModelUpdate()
			fake.modelApplyErr = errors.New("apply failed")
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetCLITestSeams(t)
			fake := installFakeGenerator(t)
			tt.configure(fake)
			capture := captureProcessOutput(t, &os.Stdout)
			err := runModelUpdate("Widget", true, false)
			_ = capture()
			if err == nil {
				t.Fatal("expected error")
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}
