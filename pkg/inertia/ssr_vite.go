package inertia

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const viteSSREndpoint = "/__inertia_ssr"

// ViteSSRRenderer posts page JSON to the @inertiajs/vite development endpoint.
type ViteSSRRenderer struct {
	endpoint        *url.URL
	client          *http.Client
	timeout         time.Duration
	maxResponseSize int64
}

// NewViteSSRRenderer creates a renderer that POSTs to Vite origin + /__inertia_ssr.
// viteDevURL may include a path prefix such as /assets/dist; that path is stripped.
func NewViteSSRRenderer(
	viteDevURL string,
	timeout time.Duration,
	maxResponseBytes int64,
) (*ViteSSRRenderer, error) {
	if timeout <= 0 {
		return nil, fmt.Errorf("inertia: SSR timeout must be positive")
	}
	if maxResponseBytes <= 0 {
		return nil, fmt.Errorf("inertia: SSR response limit must be positive")
	}

	endpoint, err := viteSSRURL(viteDevURL)
	if err != nil {
		return nil, err
	}

	return &ViteSSRRenderer{
		endpoint:        endpoint,
		client:          &http.Client{},
		timeout:         timeout,
		maxResponseSize: maxResponseBytes,
	}, nil
}

func viteSSRURL(viteDevURL string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(viteDevURL))
	if err != nil || parsed.Host == "" ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("inertia: invalid Vite dev URL")
	}

	parsed.Path = viteSSREndpoint
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed, nil
}

func useViteDevSSR(environment, viteDevURL string) bool {
	return environment == "development" && strings.TrimSpace(viteDevURL) != ""
}

// Render posts a final v3 page object to Vite's /__inertia_ssr endpoint.
func (renderer *ViteSSRRenderer) Render(ctx context.Context, page Page) (*SSRResponse, error) {
	payload, err := json.Marshal(page)
	if err != nil {
		return nil, &SSRTransportError{Kind: SSREncodeFailure, Operation: "encode page", Err: err}
	}

	bounded, cancel := context.WithTimeout(ctx, renderer.timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(
		bounded,
		http.MethodPost,
		renderer.endpoint.String(),
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, &SSRTransportError{
			Kind:      SSRTransportFailure,
			Operation: "create request",
			Err:       err,
		}
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

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

	if len(bytes.TrimSpace(body)) == 0 || bytes.Equal(bytes.TrimSpace(body), []byte("null")) {
		return nil, &SSRTransportError{
			Kind:      SSRResponseFailure,
			Operation: "render",
			Err:       fmt.Errorf("vite SSR endpoint returned no document"),
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
