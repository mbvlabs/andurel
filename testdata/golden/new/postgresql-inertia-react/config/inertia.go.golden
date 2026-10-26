package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/mbvlabs/andurel/pkg/env"
	"github.com/mbvlabs/andurel/pkg/inertia"
	"github.com/mbvlabs/andurel/pkg/validation"
)

const (
	DefaultInertiaContainerID         = "app"
	DefaultInertiaViteDevURL          = "http://localhost:5173/assets/dist"
	DefaultInertiaProtocolDebug       = true
	DefaultInertiaSSRRuntime          = "node"
	DefaultInertiaSSRBundle           = "assets/dist/ssr/ssr.js"
	DefaultInertiaSSRListen           = "http://127.0.0.1:13714"
	DefaultInertiaSSRURL              = "http://127.0.0.1:13714"
	DefaultInertiaSSRStartupTimeout   = 10 * time.Second
	DefaultInertiaSSRRequestTimeout   = 2 * time.Second
	DefaultInertiaSSRMaxResponseBytes = int64(2097152)
	DefaultInertiaSSRMinimumMajor     = 22
	DefaultInertiaSSRFailFast         = true
	DefaultInertiaEntryPoint          = "resources/js/app.tsx"
)

type Inertia struct {
	ContainerID         string
	ViteDevURL          string
	ProtocolDebug       bool
	SSRRuntime          string
	SSRBundle           string
	SSRListen           string
	SSRURL              string
	SSRStartupTimeout   time.Duration
	SSRRequestTimeout   time.Duration
	SSRMaxResponseBytes int64
	SSRMinimumMajor     int
	SSRFailFast         bool
	EntryPoint          string
}

func NewInertia() (Inertia, error) {
	e := env.New()
	cfg := Inertia{
		ContainerID:   e.String("INERTIA_CONTAINER_ID", DefaultInertiaContainerID),
		ViteDevURL:    e.String("INERTIA_VITE_DEV_URL", DefaultInertiaViteDevURL),
		ProtocolDebug: e.Bool("INERTIA_PROTOCOL_DEBUG", DefaultInertiaProtocolDebug),
		SSRRuntime:    e.String("INERTIA_SSR_RUNTIME", DefaultInertiaSSRRuntime),
		SSRBundle:     e.String("INERTIA_SSR_BUNDLE", DefaultInertiaSSRBundle),
		SSRListen:     e.String("INERTIA_SSR_LISTEN", DefaultInertiaSSRListen),
		SSRURL:        e.String("INERTIA_SSR_URL", DefaultInertiaSSRURL),
		SSRStartupTimeout: e.Duration(
			"INERTIA_SSR_STARTUP_TIMEOUT",
			DefaultInertiaSSRStartupTimeout,
		),
		SSRRequestTimeout: e.Duration(
			"INERTIA_SSR_REQUEST_TIMEOUT",
			DefaultInertiaSSRRequestTimeout,
		),
		SSRMaxResponseBytes: e.Int64(
			"INERTIA_SSR_MAX_RESPONSE_BYTES",
			DefaultInertiaSSRMaxResponseBytes,
		),
		SSRMinimumMajor: e.Int("INERTIA_SSR_MINIMUM_MAJOR", DefaultInertiaSSRMinimumMajor),
		SSRFailFast:     e.Bool("INERTIA_SSR_FAIL_FAST", DefaultInertiaSSRFailFast),
		EntryPoint:      e.String("INERTIA_ENTRY_POINT", DefaultInertiaEntryPoint),
	}

	if err := errors.Join(e.Err(), cfg.validate()); err != nil {
		return Inertia{}, fmt.Errorf("config: inertia: %w", err)
	}

	return cfg, nil
}

func (c Inertia) validate() error {
	b := validation.NewBuilder()
	b.Required("ContainerID", c.ContainerID)
	b.Required("ViteDevURL", c.ViteDevURL)
	b.Required("EntryPoint", c.EntryPoint)
	b.Required("SSRRuntime", c.SSRRuntime)
	b.Required("SSRBundle", c.SSRBundle)
	b.Required("SSRListen", c.SSRListen)
	if err := b.Err(); err != nil {
		return err
	}

	if err := inertia.ValidateSSRListen(c.SSRListen); err != nil {
		return err
	}
	if err := inertia.ValidateSSRClient(
		c.SSRURL,
		c.SSRRequestTimeout,
		c.SSRMaxResponseBytes,
	); err != nil {
		return err
	}
	if c.SSRStartupTimeout <= 0 {
		return fmt.Errorf("SSRStartupTimeout must be positive")
	}
	if c.SSRMinimumMajor <= 0 {
		return fmt.Errorf("SSRMinimumMajor must be positive")
	}

	return nil
}
