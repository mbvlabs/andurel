//go:build !linux

package cli

// verifyPortOwnedByChild is a no-op on platforms without the Linux /proc
// socket tables. The flock lease and bind preflight still apply; the ownership
// proof for the intended child is Linux-only.
func verifyPortOwnedByChild(string, int, int) error { return nil }
