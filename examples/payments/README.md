# Payments example (view-only)

This folder contains a reference implementation for adding Paddle-backed payments
into an Andurel app. It is view-only and is not wired into the generator.

Replace `example.com/myapp` with your module path before copying files.

## New files to add

- `config/paddle.go`
- `clients/payment/paddle.go`
- `controllers/payment_account.go`
- `controllers/payment_checkout.go`
- `controllers/payment_pricing.go`
- `controllers/payment_webhooks.go`
- `models/payment_customer.go`
- `models/payment_product.go`
- `models/payment_transaction.go`
- `router/routes/payment.go`
- `router/connect_payment_routes.go`
- `database/migrations/*_create_payment_customers_table.sql`
- `database/migrations/*_create_payment_products_table.sql`
- `database/migrations/*_create_payment_transactions_table.sql`
- `database/queries/payment_customers.sql`
- `database/queries/payment_products.sql`
- `database/queries/payment_transactions.sql`

## Existing files to update

- `config/config.go`
- `cmd/app/main.go`
- `.env.example`

## New files

### config/paddle.go

```go
package config

import (
    "github.com/caarlos0/env/v10"
)

type paddle struct {
    APIKey        string `env:"PADDLE_API_KEY"`
    Environment   string `env:"PADDLE_ENVIRONMENT" envDefault:"sandbox"`
    WebhookSecret string `env:"PADDLE_WEBHOOK_SECRET"`
}

func newPaddleConfig() paddle {
    cfg := paddle{}

    if err := env.ParseWithOptions(&cfg, env.Options{
        RequiredIfNoDef: true,
    }); err != nil {
        panic(err)
    }

    return cfg
}
```

### clients/payment/paddle.go

```go
package payment

import (
    "bytes"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

const (
    sandboxBaseURL    = "https://sandbox-api.paddle.com"
    productionBaseURL = "https://api.paddle.com"
)

type PaddleClient struct {
    apiKey      string
    baseURL     string
    environment string
    httpClient  *http.Client
}

func NewPaddleClient(apiKey, environment string) *PaddleClient {
    baseURL := sandboxBaseURL
    if environment == "production" {
        baseURL = productionBaseURL
    }

    return &PaddleClient{
        apiKey:      apiKey,
        baseURL:     baseURL,
        environment: environment,
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
        },
    }
}

func (c *PaddleClient) doRequest(method, path string, body interface{}, result interface{}) error {
    var reqBody io.Reader
    if body != nil {
        jsonData, err := json.Marshal(body)
        if err != nil {
            return fmt.Errorf("failed to marshal request body: %w", err)
        }
        reqBody = bytes.NewBuffer(jsonData)
    }

    req, err := http.NewRequest(method, c.baseURL+path, reqBody)
    if err != nil {
        return fmt.Errorf("failed to create request: %w", err)
    }

    req.Header.Set("Authorization", "Bearer "+c.apiKey)
    req.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("failed to execute request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        bodyBytes, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("paddle API error (status %d): %s", resp.StatusCode, string(bodyBytes))
    }

    if result != nil {
        if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
            return fmt.Errorf("failed to decode response: %w", err)
        }
    }

    return nil
}

type Product struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Description string    `json:"description"`
    TaxCategory string    `json:"tax_category"`
    ImageURL    string    `json:"image_url"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

type Price struct {
    ID          string `json:"id"`
    ProductID   string `json:"product_id"`
    Description string `json:"description"`
    UnitPrice   struct {
        Amount       string `json:"amount"`
        CurrencyCode string `json:"currency_code"`
    } `json:"unit_price"`
}

type Transaction struct {
    ID            string    `json:"id"`
    Status        string    `json:"status"`
    CustomerID    string    `json:"customer_id"`
    CurrencyCode  string    `json:"currency_code"`
    Total         string    `json:"details.totals.total"`
    CreatedAt     time.Time `json:"created_at"`
    UpdatedAt     time.Time `json:"updated_at"`
    BilledAt      time.Time `json:"billed_at"`
    InvoiceNumber string    `json:"invoice_number"`
    CheckoutURL   string    `json:"checkout.url"`
}

type ListProductsResponse struct {
    Data []Product `json:"data"`
}

type ListPricesResponse struct {
    Data []Price `json:"data"`
}

type GetTransactionResponse struct {
    Data Transaction `json:"data"`
}

func (c *PaddleClient) ListProducts() ([]Product, error) {
    var resp ListProductsResponse
    if err := c.doRequest("GET", "/products", nil, &resp); err != nil {
        return nil, err
    }
    return resp.Data, nil
}

func (c *PaddleClient) ListPrices(productID string) ([]Price, error) {
    var resp ListPricesResponse
    path := "/prices"
    if productID != "" {
        path += "?product_id=" + productID
    }
    if err := c.doRequest("GET", path, nil, &resp); err != nil {
        return nil, err
    }
    return resp.Data, nil
}

func (c *PaddleClient) GetTransaction(transactionID string) (*Transaction, error) {
    var resp GetTransactionResponse
    if err := c.doRequest("GET", "/transactions/"+transactionID, nil, &resp); err != nil {
        return nil, err
    }
    return &resp.Data, nil
}

func (c *PaddleClient) VerifyWebhookSignature(signature, requestBody, webhookSecret string) bool {
    h := hmac.New(sha256.New, []byte(webhookSecret))
    h.Write([]byte(requestBody))
    expectedSignature := hex.EncodeToString(h.Sum(nil))
    return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

type WebhookEvent struct {
    EventID    string          `json:"event_id"`
    EventType  string          `json:"event_type"`
    OccurredAt time.Time       `json:"occurred_at"`
    Data       json.RawMessage `json:"data"`
}

func ParseWebhookEvent(body []byte) (*WebhookEvent, error) {
    var event WebhookEvent
    if err := json.Unmarshal(body, &event); err != nil {
        return nil, fmt.Errorf("failed to parse webhook event: %w", err)
    }
    return &event, nil
}
```

### controllers/payment_checkout.go

```go
package controllers

import (
    "net/http"

    "example.com/myapp/clients/payment"
    "example.com/myapp/config"
    "example.com/myapp/internal/andurel"

    "github.com/labstack/echo/v5"
)

type PaymentCheckout struct {
    db     andurel.Database
    client *payment.PaddleClient
    cfg    config.Config
}

func newPaymentCheckout(db andurel.Database, client *payment.PaddleClient, cfg config.Config) PaymentCheckout {
    return PaymentCheckout{db, client, cfg}
}

func (pc PaymentCheckout) Show(c *echo.Context) error {
    // TODO: Implement checkout view
    return c.JSON(http.StatusOK, map[string]string{
        "message":     "Checkout page - implement your own view",
        "environment": pc.cfg.Paddle.Environment,
    })
}
```

### controllers/payment_pricing.go

```go
package controllers

import (
    "log/slog"
    "net/http"

    "example.com/myapp/clients/payment"
    "example.com/myapp/internal/andurel"
    "example.com/myapp/models"

    "github.com/labstack/echo/v5"
)

type PaymentPricing struct {
    db     andurel.Database
    client *payment.PaddleClient
}

func newPaymentPricing(db andurel.Database, client *payment.PaddleClient) PaymentPricing {
    return PaymentPricing{db, client}
}

func (pp PaymentPricing) Index(c *echo.Context) error {
    // Get active products from database
    products, err := models.ListActiveProducts(c.Request().Context(), pp.db.Conn())
    if err != nil {
        slog.ErrorContext(
            c.Request().Context(),
            "failed to list products",
            "error",
            err,
        )
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to load products"})
    }

    // TODO: Implement pricing view
    return c.JSON(http.StatusOK, map[string]interface{}{
        "message":  "Pricing page - implement your own view",
        "products": products,
    })
}
```

### controllers/payment_account.go

```go
package controllers

import (
    "net/http"

    "example.com/myapp/clients/payment"
    "example.com/myapp/internal/andurel"
    "example.com/myapp/models"

    "github.com/labstack/echo/v5"
)

type PaymentAccount struct {
    db     andurel.Database
    client *payment.PaddleClient
}

func newPaymentAccount(db andurel.Database, client *payment.PaddleClient) PaymentAccount {
    return PaymentAccount{db, client}
}

func (pa PaymentAccount) Index(c *echo.Context) error {
    // NOTE: This is a placeholder account page
    // Users should implement their own user-transaction linking logic
    // and query for transactions based on the logged-in user
    var transactions []models.PaymentTransaction

    // TODO: Implement account view and transaction listing
    return c.JSON(http.StatusOK, map[string]interface{}{
        "message":      "Payment account page - implement your own view",
        "transactions": transactions,
    })
}
```

### controllers/payment_webhooks.go

```go
package controllers

import (
    "encoding/json"
    "io"
    "log/slog"
    "net/http"
    "time"

    "example.com/myapp/clients/payment"
    "example.com/myapp/config"
    "example.com/myapp/internal/andurel"
    "example.com/myapp/models"

    "github.com/labstack/echo/v5"
)

type PaymentWebhooks struct {
    db     andurel.Database
    client *payment.PaddleClient
    cfg    config.Config
}

func newPaymentWebhooks(db andurel.Database, client *payment.PaddleClient, cfg config.Config) PaymentWebhooks {
    return PaymentWebhooks{db, client, cfg}
}

func (w PaymentWebhooks) Handle(c *echo.Context) error {
    // Read request body
    bodyBytes, err := io.ReadAll(c.Request().Body)
    if err != nil {
        slog.ErrorContext(
            c.Request().Context(),
            "failed to read webhook body",
            "error",
            err,
        )
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
    }

    // Verify webhook signature
    signature := c.Request().Header.Get("Paddle-Signature")
    if !w.client.VerifyWebhookSignature(signature, string(bodyBytes), w.cfg.Paddle.WebhookSecret) {
        slog.WarnContext(
            c.Request().Context(),
            "invalid webhook signature",
        )
        return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid signature"})
    }

    // Parse webhook event
    event, err := payment.ParseWebhookEvent(bodyBytes)
    if err != nil {
        slog.ErrorContext(
            c.Request().Context(),
            "failed to parse webhook event",
            "error",
            err,
        )
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid event data"})
    }

    // Handle different event types
    switch event.EventType {
    case "transaction.completed":
        if err := w.handleTransactionCompleted(c, event); err != nil {
            slog.ErrorContext(
                c.Request().Context(),
                "failed to handle transaction.completed",
                "error",
                err,
                "event_id",
                event.EventID,
            )
            return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to process event"})
        }

    case "transaction.updated":
        if err := w.handleTransactionUpdated(c, event); err != nil {
            slog.ErrorContext(
                c.Request().Context(),
                "failed to handle transaction.updated",
                "error",
                err,
                "event_id",
                event.EventID,
            )
            return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to process event"})
        }

    default:
        slog.InfoContext(
            c.Request().Context(),
            "unhandled webhook event type",
            "event_type",
            event.EventType,
            "event_id",
            event.EventID,
        )
    }

    return c.JSON(http.StatusOK, map[string]string{"status": "received"})
}

type transactionData struct {
    ID           string `json:"id"`
    Status       string `json:"status"`
    CustomerID   string `json:"customer_id"`
    CurrencyCode string `json:"currency_code"`
    Details      struct {
        Totals struct {
            Total string `json:"total"`
        } `json:"totals"`
    } `json:"details"`
    InvoiceNumber string    `json:"invoice_number"`
    BilledAt      time.Time `json:"billed_at"`
}

func (w PaymentWebhooks) handleTransactionCompleted(c *echo.Context, event *payment.WebhookEvent) error {
    var txData transactionData
    if err := json.Unmarshal(event.Data, &txData); err != nil {
        return err
    }

    // Check if transaction already exists
    _, err := models.FindPaymentTransactionByProviderID(
        c.Request().Context(),
        w.db.Conn(),
        txData.ID,
    )

    if err == nil {
        // Transaction exists, update status
        _, err = models.UpdatePaymentTransactionStatus(
            c.Request().Context(),
            w.db.Conn(),
            txData.ID,
            txData.Status,
            time.Now(),
        )
        return err
    }

    // Transaction doesn't exist - this is expected for new transactions
    // NOTE: Users should implement their own logic here to:
    // 1. Link the transaction to their user system (if applicable)
    // 2. Create the transaction record with appropriate business logic
    slog.InfoContext(
        c.Request().Context(),
        "new transaction completed",
        "transaction_id",
        txData.ID,
        "customer_id",
        txData.CustomerID,
        "status",
        txData.Status,
        "total",
        txData.Details.Totals.Total,
    )

    return nil
}

func (w PaymentWebhooks) handleTransactionUpdated(c *echo.Context, event *payment.WebhookEvent) error {
    var txData transactionData
    if err := json.Unmarshal(event.Data, &txData); err != nil {
        return err
    }

    // Update transaction status
    _, err := models.UpdatePaymentTransactionStatus(
        c.Request().Context(),
        w.db.Conn(),
        txData.ID,
        txData.Status,
        time.Now(),
    )

    if err != nil {
        slog.ErrorContext(
            c.Request().Context(),
            "failed to update transaction",
            "error",
            err,
            "transaction_id",
            txData.ID,
        )
        return err
    }

    return nil
}
```

### models/payment_customer.go

```go
package models

import (
    "context"
    "time"

    "github.com/google/uuid"

    "example.com/myapp/internal/storage"
    "example.com/myapp/models/internal/db"
)

type PaymentCustomer struct {
    ID                 uuid.UUID
    CreatedAt          time.Time
    UpdatedAt          time.Time
    ProviderCustomerID string
    Email              string
}

func FindPaymentCustomerByProviderID(
    ctx context.Context,
    exec storage.Executor,
    paddleCustomerID string,
) (PaymentCustomer, error) {
    row, err := queries.QueryPaymentCustomerByProviderID(ctx, exec, paddleCustomerID)
    if err != nil {
        return PaymentCustomer{}, err
    }

    return rowToPaymentCustomer(row)
}

type CreatePaymentCustomerData struct {
    ProviderCustomerID string
    Email              string
}

func CreatePaymentCustomer(
    ctx context.Context,
    exec storage.Executor,
    data CreatePaymentCustomerData,
) (PaymentCustomer, error) {
    params := db.InsertPaymentCustomerParams{
        ID:                 uuid.New(),
        ProviderCustomerID: data.ProviderCustomerID,
        Email:              data.Email,
    }

    row, err := queries.InsertPaymentCustomer(ctx, exec, params)
    if err != nil {
        return PaymentCustomer{}, err
    }

    return rowToPaymentCustomer(row)
}

func rowToPaymentCustomer(row db.PaymentCustomer) (PaymentCustomer, error) {
    return PaymentCustomer{
        ID:                 row.ID,
        CreatedAt:          row.CreatedAt.Time,
        UpdatedAt:          row.UpdatedAt.Time,
        ProviderCustomerID: row.ProviderCustomerID,
        Email:              row.Email,
    }, nil
}
```

### models/payment_product.go

```go
package models

import (
    "context"
    "database/sql"
    "time"

    "github.com/google/uuid"

    "example.com/myapp/internal/storage"
    "example.com/myapp/models/internal/db"
)

type PaymentProduct struct {
    ID                uuid.UUID
    CreatedAt         time.Time
    UpdatedAt         time.Time
    ProviderProductID string
    ProviderPriceID   string
    Name              string
    Description       string
    PriceAmount       string
    PriceCurrency     string
    ImageURL          string
    IsActive          bool
}

func FindPaymentProductByID(
    ctx context.Context,
    exec storage.Executor,
    id uuid.UUID,
) (PaymentProduct, error) {
    row, err := queries.QueryPaymentProductByID(ctx, exec, id)
    if err != nil {
        return PaymentProduct{}, err
    }

    return rowToPaymentProduct(row)
}

func FindPaymentProductByProviderID(
    ctx context.Context,
    exec storage.Executor,
    paddleProductID string,
) (PaymentProduct, error) {
    row, err := queries.QueryPaymentProductByProviderID(ctx, exec, paddleProductID)
    if err != nil {
        return PaymentProduct{}, err
    }

    return rowToPaymentProduct(row)
}

func ListActiveProducts(
    ctx context.Context,
    exec storage.Executor,
) ([]PaymentProduct, error) {
    rows, err := queries.QueryActiveProducts(ctx, exec)
    if err != nil {
        return nil, err
    }

    products := make([]PaymentProduct, len(rows))
    for i, row := range rows {
        product, convErr := rowToPaymentProduct(row)
        if convErr != nil {
            return nil, convErr
        }
        products[i] = product
    }

    return products, nil
}

type CreatePaymentProductData struct {
    ProviderProductID string
    ProviderPriceID   string
    Name              string
    Description       string
    PriceAmount       string
    PriceCurrency     string
    ImageURL          string
    IsActive          bool
}

func CreatePaymentProduct(
    ctx context.Context,
    exec storage.Executor,
    data CreatePaymentProductData,
) (PaymentProduct, error) {
    params := db.InsertPaymentProductParams{
        ID:                uuid.New(),
        ProviderProductID: data.ProviderProductID,
        ProviderPriceID:   data.ProviderPriceID,
        Name:              data.Name,
        Description:       sql.NullString{String: data.Description, Valid: data.Description != ""},
        PriceAmount:       data.PriceAmount,
        PriceCurrency:     data.PriceCurrency,
        ImageURL:          sql.NullString{String: data.ImageURL, Valid: data.ImageURL != ""},
        IsActive:          data.IsActive,
    }

    row, err := queries.InsertPaymentProduct(ctx, exec, params)
    if err != nil {
        return PaymentProduct{}, err
    }

    return rowToPaymentProduct(row)
}

func rowToPaymentProduct(row db.PaymentProduct) (PaymentProduct, error) {
    return PaymentProduct{
        ID:                row.ID,
        CreatedAt:         row.CreatedAt.Time,
        UpdatedAt:         row.UpdatedAt.Time,
        ProviderProductID: row.ProviderProductID,
        ProviderPriceID:   row.ProviderPriceID,
        Name:              row.Name,
        Description:       row.Description.String,
        PriceAmount:       row.PriceAmount,
        PriceCurrency:     row.PriceCurrency,
        ImageURL:          row.ImageUrl.String,
        IsActive:          row.IsActive,
    }, nil
}
```

### models/payment_transaction.go

```go
package models

import (
    "context"
    "database/sql"
    "time"

    "github.com/google/uuid"

    "example.com/myapp/internal/storage"
    "example.com/myapp/models/internal/db"
)

type PaymentTransaction struct {
    ID                    uuid.UUID
    CreatedAt             time.Time
    UpdatedAt             time.Time
    ProviderTransactionID string
    ProviderCustomerID    string
    Status                string
    Amount                string
    Currency              string
    InvoiceNumber         string
    BilledAt              time.Time
    CompletedAt           time.Time
    RawData               []byte
}

func FindPaymentTransactionByID(
    ctx context.Context,
    exec storage.Executor,
    id uuid.UUID,
) (PaymentTransaction, error) {
    row, err := queries.QueryPaymentTransactionByID(ctx, exec, id)
    if err != nil {
        return PaymentTransaction{}, err
    }

    return rowToPaymentTransaction(row)
}

func FindPaymentTransactionByProviderID(
    ctx context.Context,
    exec storage.Executor,
    paddleTransactionID string,
) (PaymentTransaction, error) {
    row, err := queries.QueryPaymentTransactionByProviderID(ctx, exec, paddleTransactionID)
    if err != nil {
        return PaymentTransaction{}, err
    }

    return rowToPaymentTransaction(row)
}

type CreatePaymentTransactionData struct {
    ProviderTransactionID string
    ProviderCustomerID    string
    Status                string
    Amount                string
    Currency              string
    InvoiceNumber         string
    BilledAt              time.Time
    CompletedAt           time.Time
    RawData               []byte
}

func CreatePaymentTransaction(
    ctx context.Context,
    exec storage.Executor,
    data CreatePaymentTransactionData,
) (PaymentTransaction, error) {
    params := db.InsertPaymentTransactionParams{
        ID:                    uuid.New(),
        ProviderTransactionID: data.ProviderTransactionID,
        ProviderCustomerID:    data.ProviderCustomerID,
        Status:                data.Status,
        Amount:                data.Amount,
        Currency:              data.Currency,
        InvoiceNumber:         sql.NullString{String: data.InvoiceNumber, Valid: data.InvoiceNumber != ""},
        BilledAt:              sql.NullTime{Time: data.BilledAt, Valid: !data.BilledAt.IsZero()},
        CompletedAt:           sql.NullTime{Time: data.CompletedAt, Valid: !data.CompletedAt.IsZero()},
        RawData:               data.RawData,
    }

    row, err := queries.InsertPaymentTransaction(ctx, exec, params)
    if err != nil {
        return PaymentTransaction{}, err
    }

    return rowToPaymentTransaction(row)
}

func UpdatePaymentTransactionStatus(
    ctx context.Context,
    exec storage.Executor,
    paddleTransactionID string,
    status string,
    completedAt time.Time,
) (PaymentTransaction, error) {
    params := db.UpdatePaymentTransactionStatusParams{
        ProviderTransactionID: paddleTransactionID,
        Status:                status,
        CompletedAt:           sql.NullTime{Time: completedAt, Valid: !completedAt.IsZero()},
    }

    row, err := queries.UpdatePaymentTransactionStatus(ctx, exec, params)
    if err != nil {
        return PaymentTransaction{}, err
    }

    return rowToPaymentTransaction(row)
}

func rowToPaymentTransaction(row db.PaymentTransaction) (PaymentTransaction, error) {
    return PaymentTransaction{
        ID:                    row.ID,
        CreatedAt:             row.CreatedAt.Time,
        UpdatedAt:             row.UpdatedAt.Time,
        ProviderTransactionID: row.ProviderTransactionID,
        ProviderCustomerID:    row.ProviderCustomerID,
        Status:                row.Status,
        Amount:                row.Amount,
        Currency:              row.Currency,
        InvoiceNumber:         row.InvoiceNumber.String,
        BilledAt:              row.BilledAt.Time,
        CompletedAt:           row.CompletedAt.Time,
        RawData:               row.RawData,
    }, nil
}
```

### router/routes/payment.go

```go
package routes

import (
    "example.com/myapp/internal/andurel"
)

const BillingPrefix = "/billing"

var PaymentWebhook = andurel.NewSimpleRoute(
    "/webhook",
    "webhook",
    BillingPrefix,
)

var PaymentCheckout = andurel.NewSimpleRoute(
    "/checkout",
    "checkout",
    BillingPrefix,
)

var PaymentPricing = andurel.NewSimpleRoute(
    "/pricing",
    "pricing",
    BillingPrefix,
)

var PaymentAccount = andurel.NewSimpleRoute(
    "/account",
    "account",
    BillingPrefix,
)
```

### router/connect_payment_routes.go

```go
package router

import (
    "errors"
    "net/http"

    "example.com/myapp/controllers"
    "example.com/myapp/router/routes"

    "github.com/labstack/echo/v5"
)

func (r Router) RegisterPaymentRoutes(
    webhooks controllers.PaymentWebhooks,
    checkout controllers.PaymentCheckout,
    pricing controllers.PaymentPricing,
    account controllers.PaymentAccount,
) error {
    errs := []error{}

    _, err := r.e.AddRoute(echo.Route{
        Method:  http.MethodPost,
        Path:    routes.PaymentWebhook.Path(),
        Name:    routes.PaymentWebhook.Name(),
        Handler: webhooks.Handle,
    })
    if err != nil {
        errs = append(errs, err)
    }

    _, err = r.e.AddRoute(echo.Route{
        Method:  http.MethodGet,
        Path:    routes.PaymentCheckout.Path(),
        Name:    routes.PaymentCheckout.Name(),
        Handler: checkout.Show,
    })
    if err != nil {
        errs = append(errs, err)
    }

    _, err = r.e.AddRoute(echo.Route{
        Method:  http.MethodGet,
        Path:    routes.PaymentPricing.Path(),
        Name:    routes.PaymentPricing.Name(),
        Handler: pricing.Index,
    })
    if err != nil {
        errs = append(errs, err)
    }

    _, err = r.e.AddRoute(echo.Route{
        Method:  http.MethodGet,
        Path:    routes.PaymentAccount.Path(),
        Name:    routes.PaymentAccount.Name(),
        Handler: account.Index,
    })
    if err != nil {
        errs = append(errs, err)
    }

    return errors.Join(errs...)
}
```

### database/migrations/00000_create_payment_customers_table.sql

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS payment_customers (
    id uuid PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
    provider_customer_id VARCHAR(255) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS payment_customers;
-- +goose StatementEnd
```

### database/migrations/00000_create_payment_products_table.sql

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS payment_products (
    id uuid PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
    provider_product_id VARCHAR(255) NOT NULL UNIQUE,
    provider_price_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price_amount VARCHAR(50) NOT NULL,
    price_currency VARCHAR(3) NOT NULL,
    image_url TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS payment_products;
-- +goose StatementEnd
```

### database/migrations/00000_create_payment_transactions_table.sql

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS payment_transactions (
    id uuid PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
    provider_transaction_id VARCHAR(255) NOT NULL UNIQUE,
    provider_customer_id VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL,
    amount VARCHAR(50) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    invoice_number VARCHAR(255),
    billed_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    raw_data JSONB
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS payment_transactions;
-- +goose StatementEnd
```

### database/queries/payment_customers.sql

```sql
-- name: QueryPaymentCustomerByProviderID :one
SELECT * FROM payment_customers WHERE provider_customer_id=$1;

-- name: InsertPaymentCustomer :one
INSERT INTO payment_customers (id, created_at, updated_at, provider_customer_id, email)
VALUES ($1, now(), now(), $2, $3)
RETURNING *;

-- name: UpdatePaymentCustomer :one
UPDATE payment_customers
SET updated_at=now(), email=$2
WHERE id = $1
RETURNING *;

-- name: DeletePaymentCustomer :exec
DELETE FROM payment_customers WHERE id=$1;
```

### database/queries/payment_products.sql

```sql
-- name: QueryPaymentProductByID :one
SELECT * FROM payment_products WHERE id=$1;

-- name: QueryPaymentProductByProviderID :one
SELECT * FROM payment_products WHERE provider_product_id=$1;

-- name: QueryActiveProducts :many
SELECT * FROM payment_products WHERE is_active=true ORDER BY created_at DESC;

-- name: QueryAllProducts :many
SELECT * FROM payment_products ORDER BY created_at DESC;

-- name: InsertPaymentProduct :one
INSERT INTO payment_products (
    id, created_at, updated_at, provider_product_id, provider_price_id,
    name, description, price_amount, price_currency, image_url, is_active
)
VALUES ($1, now(), now(), $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdatePaymentProduct :one
UPDATE payment_products
SET updated_at=now(), name=$2, description=$3, price_amount=$4,
    price_currency=$5, image_url=$6, is_active=$7
WHERE id = $1
RETURNING *;

-- name: DeletePaymentProduct :exec
DELETE FROM payment_products WHERE id=$1;
```

### database/queries/payment_transactions.sql

```sql
-- name: QueryPaymentTransactionByID :one
SELECT * FROM payment_transactions WHERE id=$1;

-- name: QueryPaymentTransactionByProviderID :one
SELECT * FROM payment_transactions WHERE provider_transaction_id=$1;

-- name: InsertPaymentTransaction :one
INSERT INTO payment_transactions (
    id, created_at, updated_at, provider_transaction_id, provider_customer_id,
    status, amount, currency, invoice_number, billed_at, completed_at, raw_data
)
VALUES ($1, now(), now(), $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: UpdatePaymentTransaction :one
UPDATE payment_transactions
SET updated_at=now(), status=$2, amount=$3, currency=$4,
    invoice_number=$5, billed_at=$6, completed_at=$7, raw_data=$8
WHERE id = $1
RETURNING *;

-- name: UpdatePaymentTransactionStatus :one
UPDATE payment_transactions
SET updated_at=now(), status=$2, completed_at=$3
WHERE provider_transaction_id = $1
RETURNING *;

-- name: DeletePaymentTransaction :exec
DELETE FROM payment_transactions WHERE id=$1;
```

## Existing files to update

### config/config.go

Add the Paddle config field and initializer:

```go
type Config struct {
    App       app
    DB        database
    Telemetry telemetry
    Paddle    paddle
}

func NewConfig() Config {
    return Config{
        App:       newAppConfig(),
        DB:        newDatabaseConfig(),
        Telemetry: newTelemetryConfig(),
        Paddle:    newPaddleConfig(),
    }
}
```

### cmd/app/main.go

Add the Paddle client and payment controller setup:

```go
import (
    "example.com/myapp/clients/payment"
)
```

```go
paddleClient := payment.NewPaddleClient(cfg.Paddle.APIKey, cfg.Paddle.Environment)
```

```go
paymentWebhooks := newPaymentWebhooks(db, paddleClient, cfg)
paymentCheckout := newPaymentCheckout(db, paddleClient, cfg)
paymentPricing := newPaymentPricing(db, paddleClient)
paymentAccount := newPaymentAccount(db, paddleClient)

if err := r.RegisterPaymentRoutes(paymentWebhooks, paymentCheckout, paymentPricing, paymentAccount); err != nil {
    return err
}
```

### .env.example

```bash
PADDLE_API_KEY=
PADDLE_WEBHOOK_SECRET=
PADDLE_ENVIRONMENT=sandbox
```

## After adding the files

- Run `sqlc generate` to refresh models.
- Run migrations to create the payment tables.
- Implement the pricing, checkout, and account views.
- Add your own user-payment linking logic in the webhook handler.
