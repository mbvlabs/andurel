package golden

import (
	"testing"

	"github.com/mbvlabs/andurel/internal/goldentest"
)

func TestSyncEmail(t *testing.T) {
	goldentest.RequireBinary(t)

	project := goldentest.CopyFixture(t, "sync_email_base")
	goldentest.RunCLI(t, project, "sync", "email")

	g := goldentest.NewGoldie(t)
	goldentest.AssertFiles(t, g, "sync/email/hello", project, []string{
		"email/hello_templ.go",
	})
}
