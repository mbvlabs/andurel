package tui

import (
	"os"

	"golang.org/x/term"
)

// Interactive is true when both stdin and stdout are terminals.
func Interactive(stdin, stdout *os.File) bool {
	if stdin == nil || stdout == nil {
		return false
	}
	return term.IsTerminal(int(stdin.Fd())) && term.IsTerminal(int(stdout.Fd()))
}

// UseTUI reports whether the Charm runner should take over the terminal.
func UseTUI(inline bool, stdin, stdout *os.File) bool {
	return !inline && Interactive(stdin, stdout)
}
