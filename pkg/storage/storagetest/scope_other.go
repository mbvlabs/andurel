//go:build !linux

package storagetest

// Other platforms cannot currently prove that PIDs are comparable.
func processScope() (string, string) { return "", "" }
