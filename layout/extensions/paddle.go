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
	builder.AddControllerImport(fmt.Sprintf("%s/clients/payment", moduleName))
	builder.AddControllerImport(fmt.Sprintf("%s/config", moduleName))

	// Add config field
	builder.AddConfigField("Paddle", "paddle")

	// Add env vars
	builder.AddEnvVar("PADDLE_API_KEY", "Paddle", "")
	builder.AddEnvVar("PADDLE_ENVIRONMENT", "Paddle", "sandbox")
	builder.AddEnvVar("PADDLE_WEBHOOK_SECRET", "Paddle", "")

	// Add Paddle client initialization
	builder.AddMainInitialization(
		"paddleClient",
		"payment.NewPaddleClient(cfg.Paddle.APIKey, cfg.Paddle.Environment)",
		"cfg",
	)

	// Add controller dependencies
	builder.AddControllerDependency("paddleClient", "*payment.PaddleClient")
	builder.AddControllerDependency("cfg", "config.Config")

	// Add controller fields
	builder.AddControllerField("PaymentWebhooks", "PaymentWebhooks")
	builder.AddControllerField("PaymentCheckout", "PaymentCheckout")
	builder.AddControllerField("PaymentPricing", "PaymentPricing")
	builder.AddControllerField("PaymentAccount", "PaymentAccount")

	// Add controller constructors
	builder.AddControllerConstructor("paymentWebhooks", "newPaymentWebhooks(db, paddleClient, cfg)")
	builder.AddControllerConstructor("paymentCheckout", "newPaymentCheckout(db, paddleClient, cfg)")
	builder.AddControllerConstructor("paymentPricing", "newPaymentPricing(db, paddleClient)")
	builder.AddControllerConstructor("paymentAccount", "newPaymentAccount(db, paddleClient)")

	// Register routes
	builder.StartRouteRegistrationFunction("registerPaymentRoutes")
	builder.AddRouteRegistration(
		"http.MethodPost",
		"routes.PaymentWebhook",
		"ctrls.PaymentWebhooks.Handle",
	)
	builder.AddRouteRegistration(
		"http.MethodGet",
		"routes.PaymentCheckout",
		"ctrls.PaymentCheckout.Show",
	)
	builder.AddRouteRegistration(
		"http.MethodGet",
		"routes.PaymentPricing",
		"ctrls.PaymentPricing.Index",
	)
	builder.AddRouteRegistration(
		"http.MethodGet",
		"routes.PaymentAccount",
		"ctrls.PaymentAccount.Index",
	)
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
	return nil
}

func (e Paddle) renderTemplates(ctx *Context) error {
	baseTime := time.Now()
	if ctx.NextMigrationTime != nil && !ctx.NextMigrationTime.IsZero() {
		baseTime = *ctx.NextMigrationTime
	}

	templates := map[string]string{
		"config_paddle.tmpl": "config/paddle.go",

		"clients_payment_paddle.tmpl": "clients/payment/paddle.go",

		"controllers_payment_webhooks.tmpl": "controllers/payment_webhooks.go",
		"controllers_payment_checkout.tmpl": "controllers/payment_checkout.go",
		"controllers_payment_pricing.tmpl":  "controllers/payment_pricing.go",
		"controllers_payment_account.tmpl":  "controllers/payment_account.go",

		"database_migrations_payment_customers.tmpl": fmt.Sprintf(
			"database/migrations/%v_create_payment_customers_table.sql",
			baseTime.Format("20060102150405"),
		),
		"database_migrations_payment_products.tmpl": fmt.Sprintf(
			"database/migrations/%v_create_payment_products_table.sql",
			baseTime.Add(1*time.Second).Format("20060102150405"),
		),
		"database_migrations_payment_transactions.tmpl": fmt.Sprintf(
			"database/migrations/%v_create_payment_transactions_table.sql",
			baseTime.Add(2*time.Second).Format("20060102150405"),
		),

		"database_queries_payment_customers.tmpl":    "database/queries/payment_customers.sql",
		"database_queries_payment_products.tmpl":     "database/queries/payment_products.sql",
		"database_queries_payment_transactions.tmpl": "database/queries/payment_transactions.sql",

		"models_payment_customer.tmpl":    "models/payment_customer.go",
		"models_payment_product.tmpl":     "models/payment_product.go",
		"models_payment_transaction.tmpl": "models/payment_transaction.go",

		"models_internal_db_payment_customer_constructors.tmpl":    "models/internal/db/payment_customer_constructors.go",
		"models_internal_db_payment_product_constructors.tmpl":     "models/internal/db/payment_product_constructors.go",
		"models_internal_db_payment_transaction_constructors.tmpl": "models/internal/db/payment_transaction_constructors.go",

		"router_routes_payment.tmpl": "router/routes/payment.go",
	}

	for tmpl, target := range templates {
		templatePath := fmt.Sprintf("templates/paddle/%s", tmpl)
		if err := ctx.ProcessTemplate(templatePath, target, nil); err != nil {
			return fmt.Errorf("failed to process %s: %w", tmpl, err)
		}
	}

	return nil
}
