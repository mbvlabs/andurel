package testseed

import "os"

const goldenFullEnv = "ANDUREL_GOLDEN_FULL"

// FullScaffold reports whether post-scaffold generators should run even when
// Enabled() is true. Nightly full-tree goldens set this via the
// andurel_golden_full build tag or ANDUREL_GOLDEN_FULL=1. PR golden builds
// leave it false so layout.Scaffold stays offline.
func FullScaffold() bool {
	if fullScaffoldBuildTag {
		return true
	}
	return os.Getenv(goldenFullEnv) == "1"
}
