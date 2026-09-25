package golden

import (
	"testing"

	"github.com/mbvlabs/andurel/v2/internal/goldentest"
)

func TestGenerateModel(t *testing.T) {
	goldentest.RequireBinary(t)

	scenarios := []struct {
		name       string
		migrations string
		// steps run in order; addMigrations overlays SQL before that step.
		steps   []generateStep
		capture []string
	}{
		{
			name:       "product_initial",
			migrations: "model_generation_initial",
			steps: []generateStep{
				{args: []string{"generate", "model", "Product", "--skip-factory"}},
			},
			capture: []string{
				"models/product.go",
				"models/model.go",
				"models/queries/product.sql",
			},
		},
		{
			name:       "product_updated",
			migrations: "model_generation_initial",
			steps: []generateStep{
				{args: []string{"generate", "model", "Product", "--skip-factory"}},
				{
					addMigrations: "model_generation_updated",
					args: []string{
						"sync",
						"model",
						"Product",
					},
				},
			},
			capture: []string{
				"models/product.go",
			},
		},
		{
			name:       "order_custom_pk",
			migrations: "model_generation_custom_pk",
			steps: []generateStep{
				{
					args: []string{
						"generate",
						"model",
						"Order",
						"--skip-factory",
						"--primary-key",
						"order_id",
					},
				},
			},
			capture: []string{
				"models/order.go",
			},
		},
		{
			name:       "audit_log_no_pk",
			migrations: "model_generation_no_pk",
			steps: []generateStep{
				{args: []string{"generate", "model", "AuditLog", "--skip-factory"}},
			},
			capture: []string{
				"models/audit_log.go",
			},
		},
		{
			name:       "event_metric_no_pk_no_uuid",
			migrations: "model_generation_no_pk_no_uuid",
			steps: []generateStep{
				{args: []string{"generate", "model", "EventMetric", "--skip-factory"}},
			},
			capture: []string{
				"models/event_metric.go",
			},
		},
		{
			name:       "product_read_only",
			migrations: "model_generation_initial",
			steps: []generateStep{
				{
					args: []string{
						"generate",
						"model",
						"Product",
						"--mode",
						"read-only",
						"--skip-factory",
					},
				},
			},
			capture: []string{
				"models/product.go",
				"models/queries/product.sql",
			},
		},
		{
			name:       "product_with_factory",
			migrations: "model_generation_initial",
			steps: []generateStep{
				{args: []string{"generate", "model", "Product"}},
			},
			capture: []string{
				"models/product.go",
				"models/factories/product.go",
				"models/queries/product.sql",
			},
		},
		{
			name: "custom",
			steps: []generateStep{
				{
					args: []string{
						"generate",
						"model",
						"AggregateResult",
						"--custom",
						"id:uuid",
						"name:string",
						"currency:int64",
					},
				},
			},
			capture: []string{
				"models/aggregate_result.go",
				"models/model.go",
				"models/queries/aggregate_result.sql",
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			project := goldentest.CopyFixture(t, "generate_base")
			if scenario.migrations != "" {
				goldentest.CopyMigrations(t, project, scenario.migrations)
			}

			g := goldentest.NewGoldie(t)
			runGenerateSteps(t, project, scenario.steps)
			goldentest.AssertFiles(t, g, "generate/model/"+scenario.name, project, scenario.capture)
		})
	}
}
