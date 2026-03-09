package naming

import (
	"runtime"
	"testing"
)

func TestBinaryName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		goos     []string
		expected string
	}{
		{
			name:     "default linux",
			input:    "shadowfax",
			goos:     []string{"linux"},
			expected: "shadowfax",
		},
		{
			name:     "explicit windows",
			input:    "shadowfax",
			goos:     []string{"windows"},
			expected: "shadowfax.exe",
		},
		{
			name:     "explicit windows uppercase",
			input:    "shadowfax",
			goos:     []string{"WINDOWS"},
			expected: "shadowfax.exe",
		},
		{
			name:     "explicit windows already has exe",
			input:    "shadowfax.exe",
			goos:     []string{"windows"},
			expected: "shadowfax.exe",
		},
		{
			name:     "explicit windows url",
			input:    "https://github.com/foo/bar/releases/download/v1/bar-windows-amd64",
			goos:     []string{"windows"},
			expected: "https://github.com/foo/bar/releases/download/v1/bar-windows-amd64.exe",
		},
		{
			name:     "explicit darwin",
			input:    "shadowfax",
			goos:     []string{"darwin"},
			expected: "shadowfax",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BinaryName(tt.input, tt.goos...)
			if got != tt.expected {
				t.Errorf("BinaryName(%q, %v) = %q, want %q", tt.input, tt.goos, got, tt.expected)
			}
		})
	}

	// Test default behavior (uses runtime.GOOS)
	t.Run("default runtime", func(t *testing.T) {
		got := BinaryName("test")
		if runtime.GOOS == "windows" {
			if got != "test.exe" {
				t.Errorf("BinaryName('test') on windows = %q, want 'test.exe'", got)
			}
		} else {
			if got != "test" {
				t.Errorf("BinaryName('test') on %s = %q, want 'test'", runtime.GOOS, got)
			}
		}
	})
}
