# Provider client

Wrap an external API in a top-level client package. Services use the client;
domains and handlers do not depend on provider-specific types.

## Files

### clients/payment.go (fragment)
```go
type PaymentClient struct {
	baseURL string
	apiKey string
	http *http.Client
}

func NewPaymentClient(baseURL, apiKey string, client *http.Client) PaymentClient {
	if baseURL == "" {
		baseURL = "https://api.provider.example"
	}
	if client == nil {
		client = http.DefaultClient
	}
	return PaymentClient{baseURL: baseURL, apiKey: apiKey, http: client}
}
```

### app/runtime.go (fragment)
```go
paymentClient := clients.NewPaymentClient(cfg.Payment.BaseURL, cfg.Payment.APIKey, nil)
```

## Rules

- Load the URL and API key in `config/`, and wire the client into services from `app/`; never read environment variables inside the client.
- Keep provider request and response types private to the adapter; expose application-owned inputs and results.
- Use request contexts, handle non-success responses and close response bodies. Do not log credentials.
- Inject an HTTP client and base URL so tests can use `httptest.Server` or a fake transport without modifying globals.

## Checklist

- [ ] Configuration and composition supply the client and its credentials.
- [ ] Tests exercise the client against a local server or fake transport, including provider errors.
- [ ] Services use application-owned types, not provider wire types.
