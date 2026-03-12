package naming

// BinaryName returns the platform-specific binary name.
// On Windows, it appends ".exe".
func BinaryName(name string) string {
	if IsWindows() {
		return name + ".exe"
	}
	return name
}
