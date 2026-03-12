package naming

import "runtime"

// GetPlatform returns the current OS and architecture.
func GetPlatform() (string, string) {
	return runtime.GOOS, runtime.GOARCH
}

// IsWindows returns true if the current OS is Windows.
func IsWindows() bool {
	return runtime.GOOS == "windows"
}
