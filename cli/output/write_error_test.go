package output

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"
)

type failingOutputWriter struct {
	err error
}

func (w failingOutputWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestProjectionOutputReturnsWriteFailures(t *testing.T) {
	wantErr := errors.New("write failed")
	for _, flag := range []string{"jq", "ids-only", "count"} {
		cmd := &cobra.Command{Use: "andurel"}
		RegisterPersistentFlags(cmd)
		cmd.SetOut(failingOutputWriter{err: wantErr})
		value := "true"
		if flag == "jq" {
			value = ".name"
		}
		if err := cmd.PersistentFlags().Set(flag, value); err != nil {
			t.Fatalf("set --%s: %v", flag, err)
		}
		data := []map[string]any{{"name": "post"}}
		if flag == "jq" {
			data = nil
			if err := OK(cmd, map[string]any{"name": "post"}, "ignored"); !errors.Is(err, wantErr) {
				t.Fatalf("--%s error = %v, want write failure", flag, err)
			}
			continue
		}
		if err := OK(cmd, data, "ignored"); !errors.Is(err, wantErr) {
			t.Fatalf("--%s error = %v, want write failure", flag, err)
		}
	}
}

func TestEnvelopeWritersReturnFailures(t *testing.T) {
	wantErr := errors.New("write failed")
	envelope := Envelope{
		OK:      true,
		Summary: "generated",
		Breadcrumbs: []Breadcrumb{{
			Command:     "andurel run",
			Description: "start server",
		}},
	}
	for _, test := range []struct {
		name string
		opts Options
	}{
		{name: "json", opts: Options{Mode: ModeJSON}},
		{name: "agent", opts: Options{Mode: ModeAgent}},
		{name: "markdown", opts: Options{Mode: ModeMarkdown}},
		{name: "human", opts: Options{Mode: ModeHuman}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := renderOK(failingOutputWriter{err: wantErr}, test.opts, envelope); !errors.Is(err, wantErr) {
				t.Fatalf("renderOK error = %v, want write failure", err)
			}
		})
	}

	cmd := &cobra.Command{Use: "andurel"}
	RegisterPersistentFlags(cmd)
	cmd.SetErr(failingOutputWriter{err: wantErr})
	if err := cmd.PersistentFlags().Set("json", "true"); err != nil {
		t.Fatalf("set JSON mode: %v", err)
	}
	if err := RenderError(cmd, errors.New("failure")); !errors.Is(err, wantErr) {
		t.Fatalf("RenderError error = %v, want write failure", err)
	}
}
