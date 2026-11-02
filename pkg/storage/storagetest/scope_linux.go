//go:build linux

package storagetest

import (
	"os"
	"regexp"
	"strings"
)

var bootIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var pidNSPattern = regexp.MustCompile(`^pid:\[[0-9]+\]$`)

// processScope identifies the kernel boot and the calling process's PID
// namespace. Never publish a partial or unrecognized scope.
func processScope() (string, string) {
	boot, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", ""
	}
	id := strings.TrimSpace(string(boot))
	ns, err := os.Readlink("/proc/self/ns/pid")
	if err != nil || !bootIDPattern.MatchString(id) || !pidNSPattern.MatchString(ns) {
		return "", ""
	}
	return id, ns
}
