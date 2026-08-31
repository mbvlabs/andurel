package inertia

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPRendererOption configures an HTTPRenderer.
type HTTPRendererOption func(*HTTPRenderer) error

// HTTPRenderer implements the official Inertia JavaScript SSR HTTP contract.
// Head and body markup returned by this renderer must be treated as trusted.
type HTTPRenderer struct {
	baseURL         *url.URL
	client          *http.Client
	timeout         time.Duration
	maxResponseSize int64
}

// NewHTTPRenderer creates a bounded SSR HTTP renderer.
func NewHTTPRenderer(config SSRConfig, options ...HTTPRendererOption) (*HTTPRenderer, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	baseURL, _ := url.Parse(strings.TrimSpace(config.URL))
	baseURL.Path = strings.TrimSuffix(strings.TrimSuffix(baseURL.Path, "/render"), "/")
	baseURL.RawQuery = ""
	baseURL.Fragment = ""

	renderer := &HTTPRenderer{
		baseURL:         baseURL,
		client:          &http.Client{},
		timeout:         config.Timeout,
		maxResponseSize: config.MaxResponseBytes,
	}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(renderer); err != nil {
			return nil, err
		}
	}
	return renderer, nil
}

// WithHTTPRendererURL overrides the SSR service URL.
func WithHTTPRendererURL(rawURL string) HTTPRendererOption {
	return func(renderer *HTTPRenderer) error {
		parsed, err := url.Parse(strings.TrimSpace(rawURL))
		if err != nil || parsed.Host == "" ||
			(parsed.Scheme != "http" && parsed.Scheme != "https") {
			return fmt.Errorf("inertia: invalid SSR URL")
		}
		parsed.Path = strings.TrimSuffix(strings.TrimSuffix(parsed.Path, "/render"), "/")
		parsed.RawQuery = ""
		parsed.Fragment = ""
		renderer.baseURL = parsed
		return nil
	}
}

// WithHTTPRendererTimeout overrides the render, health, and shutdown timeout.
func WithHTTPRendererTimeout(timeout time.Duration) HTTPRendererOption {
	return func(renderer *HTTPRenderer) error {
		if timeout <= 0 {
			return fmt.Errorf("inertia: SSR timeout must be positive")
		}
		renderer.timeout = timeout
		return nil
	}
}

// WithHTTPRendererMaxResponseBytes overrides the maximum SSR response body size.
func WithHTTPRendererMaxResponseBytes(size int64) HTTPRendererOption {
	return func(renderer *HTTPRenderer) error {
		if size <= 0 {
			return fmt.Errorf("inertia: SSR response limit must be positive")
		}
		renderer.maxResponseSize = size
		return nil
	}
}

// WithHTTPRendererClient configures the transport used by an HTTPRenderer.
func WithHTTPRendererClient(client *http.Client) HTTPRendererOption {
	return func(renderer *HTTPRenderer) error {
		if client == nil {
			return fmt.Errorf("inertia: SSR HTTP client cannot be nil")
		}
		renderer.client = client
		return nil
	}
}

// Render posts a final v3 page object to /render.
func (renderer *HTTPRenderer) Render(ctx context.Context, page Page) (*SSRResponse, error) {
	payload, err := json.Marshal(page)
	if err != nil {
		return nil, &SSRTransportError{Kind: SSREncodeFailure, Operation: "encode page", Err: err}
	}
	request, cancel, err := renderer.request(
		ctx,
		http.MethodPost,
		"/render",
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, err
	}
	defer cancel()
	request.Header.Set("Content-Type", "application/json")

	response, err := renderer.client.Do(request)
	if err != nil {
		return nil, &SSRTransportError{Kind: SSRTransportFailure, Operation: "render", Err: err}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, &SSRTransportError{
			Kind:      SSRStatusFailure,
			Operation: "render",
			Status:    response.StatusCode,
			Err:       fmt.Errorf("unexpected response status"),
		}
	}
	body, err := readBounded(response.Body, renderer.maxResponseSize)
	if err != nil {
		return nil, &SSRTransportError{
			Kind:      SSRResponseFailure,
			Operation: "read render response",
			Err:       err,
		}
	}
	var rendered SSRResponse
	if err := json.Unmarshal(body, &rendered); err != nil {
		return nil, &SSRTransportError{
			Kind:      SSRDecodeFailure,
			Operation: "decode render response",
			Err:       err,
		}
	}
	return &rendered, nil
}

// Health verifies the renderer's /health endpoint.
func (renderer *HTTPRenderer) Health(ctx context.Context) error {
	request, cancel, err := renderer.request(ctx, http.MethodGet, "/health", nil)
	if err != nil {
		return err
	}
	defer cancel()
	response, err := renderer.client.Do(request)
	if err != nil {
		return &SSRTransportError{Kind: SSRTransportFailure, Operation: "health", Err: err}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &SSRTransportError{
			Kind:      SSRStatusFailure,
			Operation: "health",
			Status:    response.StatusCode,
			Err:       fmt.Errorf("unexpected response status"),
		}
	}
	body, err := readBounded(response.Body, 64<<10)
	if err != nil {
		return &SSRTransportError{
			Kind:      SSRResponseFailure,
			Operation: "read health response",
			Err:       err,
		}
	}
	var health struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &health); err != nil {
		return &SSRTransportError{
			Kind:      SSRDecodeFailure,
			Operation: "decode health response",
			Err:       err,
		}
	}
	if !strings.EqualFold(health.Status, "ok") {
		return &SSRTransportError{
			Kind:      SSRResponseFailure,
			Operation: "health",
			Err:       fmt.Errorf("renderer is not healthy"),
		}
	}
	return nil
}

// Shutdown asks the renderer to stop through /shutdown.
func (renderer *HTTPRenderer) Shutdown(ctx context.Context) error {
	request, cancel, err := renderer.request(ctx, http.MethodPost, "/shutdown", nil)
	if err != nil {
		return err
	}
	defer cancel()
	response, err := renderer.client.Do(request)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return &SSRTransportError{Kind: SSRTransportFailure, Operation: "shutdown", Err: err}
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &SSRTransportError{
			Kind:      SSRStatusFailure,
			Operation: "shutdown",
			Status:    response.StatusCode,
			Err:       fmt.Errorf("unexpected response status"),
		}
	}
	return nil
}

func (renderer *HTTPRenderer) request(
	ctx context.Context,
	method, path string,
	body io.Reader,
) (*http.Request, context.CancelFunc, error) {
	bounded, cancel := context.WithTimeout(ctx, renderer.timeout)
	endpoint := *renderer.baseURL
	endpoint.Path = strings.TrimSuffix(endpoint.Path, "/") + path
	request, err := http.NewRequestWithContext(bounded, method, endpoint.String(), body)
	if err != nil {
		cancel()
		return nil, nil, &SSRTransportError{
			Kind:      SSRTransportFailure,
			Operation: "create request",
			Err:       err,
		}
	}
	request.Header.Set("Accept", "application/json")
	return request, cancel, nil
}

func readBounded(reader io.Reader, limit int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read SSR response: %w", err)
	}
	if int64(len(body)) > limit {
		return nil, ErrResponseTooLarge
	}
	return body, nil
}
