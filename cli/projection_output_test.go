package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/cli/output"
)

func TestCLIProjectionFiltersDataPayloadWithoutEnvelope(t *testing.T) {
	result := runCLITest(t, "commands", "--jq", ".name")
	if result.err != nil {
		t.Fatalf("commands projection: %v\nstderr:\n%s", result.err, result.stderr)
	}
	if result.stdout != "\"andurel\"\n" {
		t.Fatalf("projection output = %q", result.stdout)
	}
	if strings.Contains(result.stdout, `"ok"`) || strings.Contains(result.stdout, `"data"`) {
		t.Fatalf("projection leaked envelope: %s", result.stdout)
	}
}

func TestCLIRejectsProjectionOnUnsupportedCommandBeforeExecution(t *testing.T) {
	for _, arguments := range [][]string{
		{"run", "--jq", ".name"},
		{"run", "--ids-only"},
		{"run", "--count"},
	} {
		result := runCLITest(t, arguments...)
		if result.err == nil {
			t.Fatalf("expected %v to fail", arguments)
		}
		envelope := output.Fail(result.err)
		if envelope.Code != output.CodeUsage || !strings.Contains(envelope.Error, "not supported") {
			t.Fatalf("%v error = %#v", arguments, envelope)
		}
	}
}

func TestOrdinaryJSONAndAgentOutputRetainStableEnvelopes(t *testing.T) {
	for _, mode := range []string{"--json", "--agent"} {
		result := runCLITest(t, "commands", mode)
		if result.err != nil {
			t.Fatalf("commands %s: %v", mode, result.err)
		}
		var envelope struct {
			OK   bool            `json:"ok"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal([]byte(result.stdout), &envelope); err != nil {
			t.Fatalf("decode %s envelope: %v\n%s", mode, err, result.stdout)
		}
		if !envelope.OK || len(envelope.Data) == 0 {
			t.Fatalf("unstable %s envelope: %#v", mode, envelope)
		}
	}
}

func TestStructuredMutationSuppressesHumanProgressOnStdoutAndStderr(t *testing.T) {
	resetCLITestSeams(t)
	fake := &fakeGenerator{
		onGenerateModel: func() {
			_, _ = fmt.Fprintln(os.Stdout, "human progress")
			_, _ = fmt.Fprintln(os.Stderr, "human warning")
		},
	}
	newGenerator = func() (cliGenerator, error) { return fake, nil }

	result := executeCLITest(t, "generate", "model", "Product", "--json")
	if result.err != nil {
		t.Fatalf("structured mutation: %v\nstderr:\n%s", result.err, result.stderr)
	}
	if strings.Contains(result.stdout, "human progress") || strings.Contains(result.stderr, "human warning") {
		t.Fatalf("structured mutation leaked human output\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
	var envelope output.Envelope
	if err := json.Unmarshal([]byte(result.stdout), &envelope); err != nil || !envelope.OK {
		t.Fatalf("decode mutation envelope: %v\n%s", err, result.stdout)
	}
}
