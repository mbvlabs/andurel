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
