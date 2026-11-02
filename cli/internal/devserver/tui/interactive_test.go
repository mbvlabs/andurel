package tui

import (
	"os"
	"testing"
)

func TestUseTUI(t *testing.T) {
	if UseTUI(true, os.Stdin, os.Stdout) {
		t.Fatal("--inline should disable the TUI even on a TTY")
	}
}

func TestInteractiveNilFiles(t *testing.T) {
	if Interactive(nil, os.Stdout) || Interactive(os.Stdin, nil) {
		t.Fatal("nil files are not interactive")
	}
}
