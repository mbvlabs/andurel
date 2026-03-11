package naming

import (
	"runtime"
)

// BinaryName returns the platform-specific binary name.
// On Windows, it appends ".exe".
func BinaryName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}
