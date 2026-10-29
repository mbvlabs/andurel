package cli

import (
	"os"
	"strings"
)

// envWithOverrides returns the process environment with the named keys
// replaced. Duplicate keys are removed so a child's os.Getenv sees the value
// the caller intended.
func envWithOverrides(overrides map[string]string) []string {
	return envWithBase(os.Environ(), overrides)
}

func envWithBase(base []string, overrides map[string]string) []string {
	env := make([]string, 0, len(base)+len(overrides))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if ok {
			if _, overridden := overrides[key]; overridden {
				continue
			}
		}
		env = append(env, entry)
	}
	for key, value := range overrides {
		env = append(env, key+"="+value)
	}
	return env
}
