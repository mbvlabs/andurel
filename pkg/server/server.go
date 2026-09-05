// Package server provides an HTTP server with graceful shutdown capabilities.
package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"slices"
	"time"
)

const (
	DevEnvironment  = "development"
	ProdEnvironment = "production"
	TestEnvironment = "testing"
)

type Shutdowner interface {
	Shutdown(ctx context.Context) error
}

type Server struct {
	srv         *http.Server
	Shutdowners []Shutdowner
}

type ServerOptions struct {
	IdleTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type ServerOption func(*ServerOptions)

// DefaultConfig returns bounded HTTP timeouts suitable for ordinary requests.
func DefaultConfig() ServerOptions {
	return ServerOptions{IdleTimeout: 120 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 30 * time.Second}
}

// Validate checks HTTP timeout values. Zero explicitly disables a timeout.
func (config ServerOptions) Validate() error {
	if config.IdleTimeout < 0 || config.ReadTimeout < 0 || config.WriteTimeout < 0 {
		return fmt.Errorf("server: HTTP timeouts cannot be negative")
	}
	return nil
}

func WithTimeouts(idle, read, write time.Duration) ServerOption {
	return func(options *ServerOptions) {
		options.IdleTimeout = idle
		options.ReadTimeout = read
		options.WriteTimeout = write
	}
}

func New(
	ctx context.Context,
	host string,
	port string,
	env string,
	handler http.Handler,
	shutdowners []Shutdowner,
	options ...ServerOption,
) Server {
	serverOptions := DefaultConfig()
	for _, option := range options {
		if option != nil {
			option(&serverOptions)
		}
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf("%v:%v", host, port),
		Handler:      handler,
		IdleTimeout:  serverOptions.IdleTimeout,
		ReadTimeout:  serverOptions.ReadTimeout,
		WriteTimeout: serverOptions.WriteTimeout,
		BaseContext:  func(_ net.Listener) context.Context { return ctx },
	}

	server := Server{
		srv:         srv,
		Shutdowners: slices.Clone(shutdowners),
	}

	server.Shutdowners = append(server.Shutdowners, srv)

	return server
}

func (s *Server) Start(
	_ context.Context,
	_ string,
) error {
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server: listen: %w", err)
	}

	return nil
}
