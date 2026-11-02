//go:build unix

package storagetest

import (
	"errors"
	"syscall"
	"testing"
)

func TestProcessCheckAliveUncertainty(t *testing.T) {
	for _, err := range []error{nil, syscall.EPERM, syscall.EACCES, errors.New("ambiguous")} {
		if !processCheckAlive(err) {
			t.Fatalf("error %v treated as dead", err)
		}
	}
	if processCheckAlive(syscall.ESRCH) {
		t.Fatal("ESRCH treated as alive")
	}
}
