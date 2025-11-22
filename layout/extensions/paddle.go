package extensions

import (
	"fmt"
	"time"

	"github.com/mbvlabs/andurel/layout/cmds"
)

type Paddle struct{}

func (e Paddle) Name() string {
	return "paddle"
}

func (e Paddle) Apply(ctx *Context) error {
	if ctx == nil || ctx.Data == nil {
		return fmt.Errorf("paddle: context or data is nil")
	}

	builder := ctx.Builder()
	moduleName := ctx.Data.GetModuleName()

	// Add imports
	builder.AddMainImport(fmt.Sprintf("%s/clients/payment", moduleName))
	builder.AddControllerImport(fmt.Sprintf("%s/config", moduleName))

	// Add config field
	builder.AddConfigField("Paddle", "paddle")

	// Add Paddle client initialization
	builder.AddMainInitialization(
		"paddleClient",
		"payment.NewPaddleClient(cfg.Paddle.ApiKey, cfg.Paddle.Environment)",
		"cfg",
	)

	// Add controller dependencies
	builder.AddControllerDependency("paddleClient", "*payment.PaddleClient")
	builder.AddControllerDependency("cfg", "config.Config")

	// Add controller fields
	builder.AddControllerField("PaddleWebhooks", "PaddleWebhooks")
	builder.AddControllerField("PaddleCheckout", "PaddleCheckout")
	builder.AddControllerField("PaddlePricing", "PaddlePricing")
	builder.AddControllerField("PaddleAccount", "PaddleAccount")

	// Add controller constructors
	builder.AddControllerConstructor("paddleWebhooks", "newPaddleWebhooks(db, paddleClient, cfg)")
	builder.AddControllerConstructor("paddleCheckout", "newPaddleCheckout(db, paddleClient, cfg)")
	builder.AddControllerConstructor("paddlePricing", "newPaddlePricing(db, paddleClient)")
	builder.AddControllerConstructor("paddleAccount", "newPaddleAccount(db, paddleClient)")

	// Register routes
	builder.StartRouteRegistrationFunction("registerPaddleRoutes")
	builder.AddRouteRegistration("http.MethodPost", "routes.PaddleWebhook", "ctrls.PaddleWebhooks.Handle")
	builder.AddRouteRegistration("http.MethodGet", "routes.PaddleCheckout", "ctrls.PaddleCheckout.Show")
	builder.AddRouteRegistration("http.MethodGet", "routes.PaddlePricing", "ctrls.PaddlePricing.Index")
	builder.AddRouteRegistration("http.MethodGet", "routes.PaddleAccount", "ctrls.PaddleAccount.Index")
	builder.EndRouteRegistrationFunction()

	if err := e.renderTemplates(ctx); err != nil {
		return fmt.Errorf("paddle: failed to render templates: %w", err)
	}

	ctx.AddPostStep(func(targetDir string) error {
		return cmds.RunSqlcGenerate(targetDir)
	})

	return nil
}

func (e Paddle) Dependencies() []string {
	return []string{"auth"}
}

func (e Paddle) renderTemplates(ctx *Context) error {
	baseTime := time.Now()
	if ctx.NextMigrationTime != nil && !ctx.NextMigrationTime.IsZero() {
		baseTime = *ctx.NextMigrationTime
	}

	templates := map[string]string{
		"config_paddle.tmpl": "config/paddle.go",

		"clients_payment_paddle.tmpl": "clients/payment/paddle.go",

		"controllers_paddle_webhooks.tmpl": "controllers/paddle_webhooks.go",
		"controllers_paddle_checkout.tmpl": "controllers/paddle_checkout.go",
		"controllers_paddle_pricing.tmpl":  "controllers/paddle_pricing.go",
		"controllers_paddle_account.tmpl":  "controllers/paddle_account.go",

		"database_migrations_paddle_customers.tmpl": fmt.Sprintf(
			"database/migrations/%v_create_paddle_customers_table.sql",
			baseTime.Format("20060102150405"),
		),
		"database_migrations_paddle_products.tmpl": fmt.Sprintf(
			"database/migrations/%v_create_paddle_products_table.sql",
			baseTime.Add(1*time.Second).Format("20060102150405"),
		),
		"database_migrations_paddle_transactions.tmpl": fmt.Sprintf(
			"database/migrations/%v_create_paddle_transactions_table.sql",
			baseTime.Add(2*time.Second).Format("20060102150405"),
		),

		"database_queries_paddle_customers.tmpl":    "database/queries/paddle_customers.sql",
		"database_queries_paddle_products.tmpl":     "database/queries/paddle_products.sql",
		"database_queries_paddle_transactions.tmpl": "database/queries/paddle_transactions.sql",

		"models_paddle_customer.tmpl":    "models/paddle_customer.go",
		"models_paddle_product.tmpl":     "models/paddle_product.go",
		"models_paddle_transaction.tmpl": "models/paddle_transaction.go",

		"models_internal_db_paddle_customer_constructors.tmpl":    "models/internal/db/paddle_customer_constructors.go",
		"models_internal_db_paddle_product_constructors.tmpl":     "models/internal/db/paddle_product_constructors.go",
		"models_internal_db_paddle_transaction_constructors.tmpl": "models/internal/db/paddle_transaction_constructors.go",

		"router_routes_paddle.tmpl": "router/routes/paddle.go",

		"views_paddle_checkout.tmpl": "views/paddle_checkout.templ",
		"views_paddle_pricing.tmpl":  "views/paddle_pricing.templ",
		"views_paddle_account.tmpl":  "views/paddle_account.templ",
	}

	for tmpl, target := range templates {
		templatePath := fmt.Sprintf("templates/paddle/%s", tmpl)
		if err := ctx.ProcessTemplate(templatePath, target, nil); err != nil {
			return fmt.Errorf("failed to process %s: %w", tmpl, err)
		}
	}

	return nil
}
