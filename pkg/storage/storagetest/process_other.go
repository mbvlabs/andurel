//go:build !unix && !windows

package storagetest

// processAlive cannot check other processes on this platform, so it reports
// every PID as alive and leftover containers are never removed.
func processAlive(int) bool {
	return true
}
