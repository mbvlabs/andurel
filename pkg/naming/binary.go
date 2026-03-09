package naming

import (
	"runtime"
	"strings"
)

// BinaryName returns the platform-specific name for a binary.
//
// By default, it appends ".exe" if the current system is Windows (runtime.GOOS).
// You can optionally provide a target OS (e.g., "windows", "linux") as the second
// argument to resolve the binary name for a remote platform. This is crucial
// for cross-platform tool synchronization and downloading.
//
// Usage:
//   naming.BinaryName("shadowfax")           // Uses current OS (e.g. "shadowfax" on Linux, "shadowfax.exe" on Windows)
//   naming.BinaryName("shadowfax", "windows") // Always returns "shadowfax.exe" regardless of current OS
func BinaryName(name string, goos ...string) string {
	targetOS := runtime.GOOS
	if len(goos) > 0 {
		targetOS = goos[0]
	}

	if targetOS == "windows" {
		if !strings.HasSuffix(strings.ToLower(name), ".exe") {
			return name + ".exe"
		}
	}
	return name
}
