package golden

import (
	"testing"

	"github.com/mbvlabs/andurel/v2/internal/goldentest"
)

func TestGenerateQuery(t *testing.T) {
	goldentest.RequireBinary(t)

	scenarios := []struct {
		name    string
		args    []string
		capture []string
	}{
		{
			name: "named",
			args: []string{"generate", "query", "UserReport"},
			capture: []string{
				"models/queries/user_report.sql",
			},
		},
		{
			name: "with_table",
			args: []string{"generate", "query", "WidgetReport", "--table", "widgets"},
			capture: []string{
				"models/queries/widget_report.sql",
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			project := goldentest.CopyFixture(t, "generate_base")
			g := goldentest.NewGoldie(t)
			goldentest.RunCLI(t, project, scenario.args...)
			goldentest.AssertFiles(t, g, "generate/query/"+scenario.name, project, scenario.capture)
		})
	}
}
