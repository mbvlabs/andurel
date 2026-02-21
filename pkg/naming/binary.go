package naming

import (
	"runtime"
	"strings"
)

// BinaryName appends .exe to the given binary name if running on Windows.
func BinaryName(name string) string {
	if runtime.GOOS == "windows" {
		if !strings.HasSuffix(strings.ToLower(name), ".exe") {
			return name + ".exe"
		}
	}
	return name
}
