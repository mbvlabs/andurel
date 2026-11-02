package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVerifyHelpDisclosesHostFallbackPolicyLimit(t *testing.T) {
	cmd := NewRootCommand("test", "test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"verify", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Plain verify does not statically check missing host fallback policies",
		"--with-tests",
		"application startup test runs",
		"Scoped or skipped tests may not exercise startup",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("verify help missing %q:\n%s", want, out.String())
		}
	}
}
