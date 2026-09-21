package config

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/mbvlabs/andurel/pkg/validation"
)

const (
	DefaultHTTPHost         = "localhost"
	DefaultHTTPPort         = "8080"
	DefaultCSRFStrategy     = "header_only"
	DefaultHTTPIdleTimeout  = 120 * time.Second
	DefaultHTTPReadTimeout  = 10 * time.Second
	DefaultHTTPWriteTimeout = 30 * time.Second
)

type HTTP struct {
	Host               string
	Port               string
	CORSAllowedOrigins []string
	CSRFStrategy       string
	CSRFTrustedOrigins []string
	IdleTimeout        time.Duration
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
}

func NewHTTP() (HTTP, error) {
	env := newEnvironment()
	cfg := HTTP{
		Host:               env.String("HOST", DefaultHTTPHost),
		Port:               env.String("PORT", DefaultHTTPPort),
		CORSAllowedOrigins: env.Strings("CORS_ALLOWED_ORIGINS", nil),
		CSRFStrategy:       env.String("CSRF_STRATEGY", DefaultCSRFStrategy),
		CSRFTrustedOrigins: env.Strings("CSRF_TRUSTED_ORIGINS", nil),
		IdleTimeout:        env.Duration("HTTP_IDLE_TIMEOUT", DefaultHTTPIdleTimeout),
		ReadTimeout:        env.Duration("HTTP_READ_TIMEOUT", DefaultHTTPReadTimeout),
		WriteTimeout:       env.Duration("HTTP_WRITE_TIMEOUT", DefaultHTTPWriteTimeout),
	}

	if err := errors.Join(env.Err(), cfg.validate()); err != nil {
		return HTTP{}, fmt.Errorf("config: http: %w", err)
	}

	return cfg, nil
}

func (c HTTP) validate() error {
	b := validation.NewBuilder()
	b.Required("Host", c.Host)
	b.Required("Port", c.Port)
	b.Required("CSRFStrategy", c.CSRFStrategy)
	b.MinInt("IdleTimeout", int64(c.IdleTimeout), 0)
	b.MinInt("ReadTimeout", int64(c.ReadTimeout), 0)
	b.MinInt("WriteTimeout", int64(c.WriteTimeout), 0)
	if err := b.Err(); err != nil {
		return err
	}
	port, err := strconv.Atoi(c.Port)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("PORT must be between 1 and 65535")
	}
	switch c.CSRFStrategy {
	case "header_only", "header_or_legacy_token":
	default:
		return fmt.Errorf("unsupported CSRF_STRATEGY %q", c.CSRFStrategy)
	}

	return nil
}

func (c HTTP) Clone() HTTP {
	c.CORSAllowedOrigins = slices.Clone(c.CORSAllowedOrigins)
	c.CSRFTrustedOrigins = slices.Clone(c.CSRFTrustedOrigins)
	return c
}
