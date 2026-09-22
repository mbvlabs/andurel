package golden

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mbvlabs/andurel/internal/goldentest"
)

// exitGeneration matches cli/output.ExitGeneration (stale --check).
const exitGeneration = 5

func TestSyncFactory(t *testing.T) {
	goldentest.RequireBinary(t)

	t.Run("preserve_custom", func(t *testing.T) {
		project := goldentest.CopyFixture(t, "sync_factory_base")
		g := goldentest.NewGoldie(t)

		goldentest.RunCLI(t, project, "sync", "factory", "Widget", "--sync")
		goldentest.AssertFiles(t, g, "sync/factory/preserve_custom", project, []string{
			"models/factories/widget.go",
		})
	})

	t.Run("create_missing", func(t *testing.T) {
		project := goldentest.CopyFixture(t, "sync_factory_base")
		g := goldentest.NewGoldie(t)

		goldentest.AssertMissing(t, project, "models/factories/product.go")
		goldentest.RunCLI(t, project, "sync", "factory", "Product", "--sync")
		goldentest.AssertFiles(t, g, "sync/factory/create_missing", project, []string{
			"models/factories/product.go",
		})
	})

	t.Run("idempotent", func(t *testing.T) {
		project := goldentest.CopyFixture(t, "sync_factory_base")
		g := goldentest.NewGoldie(t)

		goldentest.RunCLI(t, project, "sync", "factory", "Widget", "--sync")
		goldentest.RunCLI(t, project, "sync", "factory", "Widget", "--sync")
		goldentest.AssertFiles(t, g, "sync/factory/preserve_custom", project, []string{
			"models/factories/widget.go",
		})
	})
}

func TestSyncFactories(t *testing.T) {
	goldentest.RequireBinary(t)

	project := goldentest.CopyFixture(t, "sync_factory_base")
	g := goldentest.NewGoldie(t)

	goldentest.RunCLI(t, project, "sync", "factories", "--sync")
	goldentest.AssertFiles(t, g, "sync/factories/bulk", project, []string{
		"models/factories/widget.go",
		"models/factories/product.go",
	})
}

func TestSyncFactoryCheck(t *testing.T) {
	goldentest.RequireBinary(t)

	t.Run("stale_exits_without_write", func(t *testing.T) {
		project := goldentest.CopyFixture(t, "sync_factory_base")
		before, err := os.ReadFile(filepath.Join(project, "models", "factories", "widget.go"))
		if err != nil {
			t.Fatalf("read fixture factory: %v", err)
		}

		out, exitCode := goldentest.RunCLIExit(t, project, "sync", "factory", "Widget", "--check")
		if exitCode != exitGeneration {
			t.Fatalf(
				"expected exit %d for stale factory, got %d\n%s",
				exitGeneration,
				exitCode,
				out,
			)
		}
		goldentest.AssertFileBytesEquals(t, project, "models/factories/widget.go", before)
	})

	t.Run("clean_after_sync", func(t *testing.T) {
		project := goldentest.CopyFixture(t, "sync_factory_base")

		goldentest.RunCLI(t, project, "sync", "factory", "Widget", "--sync")
		out, exitCode := goldentest.RunCLIExit(t, project, "sync", "factory", "Widget", "--check")
		if exitCode != 0 {
			t.Fatalf("expected exit 0 for up-to-date factory, got %d\n%s", exitCode, out)
		}
	})
}

// JSON stdout goldens are skipped: FactorySyncResult.Path is absolute and
// embeds the temp project directory, so --check --json bytes are not stable.
