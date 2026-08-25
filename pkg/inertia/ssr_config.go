package inertia

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	// DefaultSSRURL is the loopback address used by the official Inertia SSR server.
	DefaultSSRURL = "http://127.0.0.1:13714"
	// DefaultSSRTimeout bounds render, health, and shutdown HTTP operations.
	DefaultSSRTimeout = 2 * time.Second
	// DefaultSSRMaxResponseBytes bounds trusted SSR markup returned by a renderer.
	DefaultSSRMaxResponseBytes int64 = 2 << 20
)

// ErrResponseTooLarge reports an SSR response that exceeds SSRConfig.MaxResponseBytes.
var ErrResponseTooLarge = errors.New("inertia: SSR response exceeds configured limit")

// SSRConfig controls communication with an Inertia SSR HTTP service.
type SSRConfig struct {
	URL              string
	Timeout          time.Duration
	MaxResponseBytes int64
}

// DefaultSSRConfig returns loopback, timeout, and response-size defaults.
func DefaultSSRConfig() SSRConfig {
	return SSRConfig{
		URL:              DefaultSSRURL,
		Timeout:          DefaultSSRTimeout,
		MaxResponseBytes: DefaultSSRMaxResponseBytes,
	}
}

// Validate verifies HTTP renderer configuration.
func (config SSRConfig) Validate() error {
	parsed, err := url.Parse(strings.TrimSpace(config.URL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("inertia: invalid SSR URL")
	}
	if config.Timeout <= 0 {
		return fmt.Errorf("inertia: SSR timeout must be positive")
	}
	if config.MaxResponseBytes <= 0 {
		return fmt.Errorf("inertia: SSR response limit must be positive")
	}
	return nil
}

// SSRTransportErrorKind identifies an SSR transport failure category.
type SSRTransportErrorKind string

const (
	SSRTransportFailure SSRTransportErrorKind = "transport"
	SSRStatusFailure    SSRTransportErrorKind = "status"
	SSREncodeFailure    SSRTransportErrorKind = "encode"
	SSRDecodeFailure    SSRTransportErrorKind = "decode"
	SSRResponseFailure  SSRTransportErrorKind = "response"
)

// SSRTransportError describes an SSR transport operation failure.
type SSRTransportError struct {
	Kind      SSRTransportErrorKind
	Operation string
	Status    int
	Err       error
}

func (err *SSRTransportError) Error() string {
	if err == nil {
		return "<nil>"
	}
	if err.Status != 0 {
		return fmt.Sprintf("inertia SSR %s: HTTP %d: %v", err.Operation, err.Status, err.Err)
	}
	return fmt.Sprintf("inertia SSR %s: %v", err.Operation, err.Err)
}

func (err *SSRTransportError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Err
}
