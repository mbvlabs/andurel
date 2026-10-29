//go:build !linux

package cli

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestVerifyPortOwnedByChildFailsClosedOffLinux(t *testing.T) {
	err := verifyPortOwnedByChild("127.0.0.1", 13714, os.Getpid())
	if err == nil {
		t.Fatal("unsupported platforms must not accept unproven listener ownership")
	}
	if !strings.Contains(err.Error(), runtime.GOOS) ||
		!strings.Contains(err.Error(), "refusing to publish") {
		t.Fatalf("ownership error is not actionable: %v", err)
	}
}
