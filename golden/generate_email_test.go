package golden

import (
	"testing"

	"github.com/mbvlabs/andurel/v2/internal/goldentest"
)

func TestGenerateEmail(t *testing.T) {
	goldentest.RequireBinary(t)

	project := goldentest.CopyFixture(t, "generate_base")
	goldentest.RunCLI(t, project, "generate", "email", "WelcomeEmail")

	g := goldentest.NewGoldie(t)
	goldentest.AssertFiles(t, g, "generate/email/welcome_email", project, []string{
		"email/welcome_email.templ",
	})
}
