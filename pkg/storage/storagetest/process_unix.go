//go:build unix

package storagetest

import (
	"errors"
	"syscall"
)

// processAlive reports whether pid is running on this host, the same check as
// `kill -0`. EPERM means the process exists but belongs to another user. Any
// error other than ESRCH counts as alive, so an unclear answer never removes a
// container.
func processAlive(pid int) bool {
	return processCheckAlive(syscall.Kill(pid, 0))
}

func processCheckAlive(err error) bool {
	return err == nil || !errors.Is(err, syscall.ESRCH)
}
