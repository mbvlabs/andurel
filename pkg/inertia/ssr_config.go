package inertia

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// ErrResponseTooLarge reports an SSR response that exceeds the configured limit.
var ErrResponseTooLarge = errors.New("inertia: SSR response exceeds configured limit")

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

// ValidateSSRClient verifies the HTTP client used by cmd/app to POST /render.
// The URL may be any http(s) host, including a service DNS name.
func ValidateSSRClient(ssrURL string, timeout time.Duration, maxResponseBytes int64) error {
	parsed, err := url.Parse(strings.TrimSpace(ssrURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("inertia: invalid SSR URL")
	}

	if timeout <= 0 {
		return fmt.Errorf("inertia: SSR timeout must be positive")
	}

	if maxResponseBytes <= 0 {
		return fmt.Errorf("inertia: SSR response limit must be positive")
	}

	return nil
}

// ValidateSSRListen verifies the URL Node binds. Host must be an IP or localhost;
// service hostnames cannot be bound.
func ValidateSSRListen(listenURL string) error {
	_, err := parseSSRListen(listenURL)
	return err
}

type ssrListen struct {
	host      string
	port      string
	healthURL string
}

func parseSSRListen(raw string) (ssrListen, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || parsed.Scheme != "http" {
		return ssrListen{}, fmt.Errorf("inertia: SSR listen URL must be an HTTP URL")
	}

	host := parsed.Hostname()
	port := parsed.Port()
	if port == "" {
		return ssrListen{}, fmt.Errorf("inertia: SSR listen URL must include a port")
	}
	if !validSSRListenHost(host) {
		return ssrListen{}, fmt.Errorf(
			"inertia: SSR listen URL must bind an IP address or localhost, not a service hostname",
		)
	}

	healthHost := host
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		if ip.To4() != nil {
			healthHost = "127.0.0.1"
		} else {
			healthHost = "::1"
		}
	}

	return ssrListen{
		host:      host,
		port:      port,
		healthURL: "http://" + net.JoinHostPort(healthHost, port),
	}, nil
}

func validSSRListenHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}

	return net.ParseIP(host) != nil
}
