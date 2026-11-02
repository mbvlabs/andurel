//go:build windows

package storagetest

import (
	"errors"
	"os"
	"syscall"
)

// errorInvalidParameter is the Windows error OpenProcess returns for a PID that
// no process has.
const errorInvalidParameter syscall.Errno = 87

// processAlive reports whether pid is running on this host. Only "no such
// process" counts as exited; any other error, such as access denied, counts as
// alive, so an unclear answer never removes a container.
func processAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return !errors.Is(err, errorInvalidParameter)
	}
	_ = process.Release()
	return true
}
