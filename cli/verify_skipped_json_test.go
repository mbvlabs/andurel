package cli

import (
	"strings"
	"testing"
)

// B7-003: a skipped stage that has nothing to check is not a failure. The
// command-level envelope stays ok:true while the stage records
// status:skipped, ok:false and not_applicable:true. Consumers must inspect the
// stage status instead of treating the envelope as proof that every check ran.
func TestVerifySkippedContentKeepsStructuredEnvelopeSemantics(t *testing.T) {
	root := newVerifyTestProject(t)

	result := runVerifyInProject(t, root, "verify", "--only", "content", "--json")
	if result.err != nil {
		t.Fatalf("verify --only content: %v\n%s", result.err, result.stderr)
	}

	envelope := decodeVerifyEnvelope(t, result.stdout)
	if !envelope.OK {
		t.Fatalf("envelope ok = false, want true for a skipped stage:\n%s", result.stdout)
	}
	if !envelope.Data.OK {
		t.Fatalf("report ok = false, want true for a skipped stage:\n%s", result.stdout)
	}
	step := findVerifyStep(t, envelope.Data, "content")
	if step.Status != verifyStatusSkipped {
		t.Fatalf("content status = %q, want %q", step.Status, verifyStatusSkipped)
	}
	if step.OK {
		t.Fatalf("content step ok = true, want false: %#v", step)
	}
	if !step.NotApplicable {
		t.Fatalf("content step not_applicable = false, want true: %#v", step)
	}
	for _, want := range []string{
		`"ok": true`,
		`"status": "skipped"`,
		`"ok": false`,
		`"not_applicable": true`,
	} {
		if !strings.Contains(result.stdout, want) {
			t.Fatalf("structured output missing %s:\n%s", want, result.stdout)
		}
	}
}
