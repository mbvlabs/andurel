//go:build !linux

package cli

import (
	"fmt"
	"runtime"
)

// verifyPortOwnedByChild fails closed on platforms without the Linux /proc
// socket tables. The flock lease and bind preflight still prevent two
// launchers from choosing the same free port, but they cannot prove that the
// process answering /health is the intended child rather than a process that
// won the bind race. Publishing an unverified URL is worse than refusing to
// start.
func verifyPortOwnedByChild(host string, port, childPID int) error {
	return fmt.Errorf(
		"cannot prove that child pid %d owns the listener on %s:%d on %s: socket-ownership proof requires Linux /proc; refusing to publish an unverified SSR URL (run the child directly with a fixed INERTIA_SSR_LISTEN instead)",
		childPID,
		host,
		port,
		runtime.GOOS,
	)
}
