package anduin

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Process environments recognised by the server.
const (
	DevEnvironment  = "development"
	ProdEnvironment = "production"
	TestEnvironment = "testing"
)

// Shutdowner is anything the Server shuts down gracefully on stop (the HTTP
// listener plus application components).
type Shutdowner interface {
	Shutdown(ctx context.Context) error
}

// ServerOptions holds the bounded HTTP timeouts. A zero value disables a bound.
type ServerOptions struct {
	IdleTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// ServerOption mutates ServerOptions.
type ServerOption func(*ServerOptions)

// DefaultConfig returns bounded HTTP timeouts suitable for ordinary requests.
func DefaultConfig() ServerOptions {
	return ServerOptions{
		IdleTimeout:  120 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
}

// Validate checks HTTP timeout values. Zero explicitly disables a timeout.
func (config ServerOptions) Validate() error {
	if config.IdleTimeout < 0 || config.ReadTimeout < 0 || config.WriteTimeout < 0 {
		return fmt.Errorf("anduin: HTTP timeouts cannot be negative")
	}
	return nil
}

// WithTimeouts sets idle, read, and write timeouts.
func WithTimeouts(idle, read, write time.Duration) ServerOption {
	return func(options *ServerOptions) {
		options.IdleTimeout = idle
		options.ReadTimeout = read
		options.WriteTimeout = write
	}
}

// Server owns an HTTP listener. Router owns dispatch; Server owns the
// listener lifecycle.
type Server struct {
	srv *http.Server
}

// NewServer builds a Server for handler listening on host:port. ctx is the
// base context of every request.
func NewServer(
	ctx context.Context,
	host string,
	port string,
	handler http.Handler,
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

	return Server{srv: srv}
}

// Addr returns the listener address.
func (s *Server) Addr() string { return s.srv.Addr }

// Start serves HTTP until Shutdown is called or the listener fails.
func (s *Server) Start() error {
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("anduin: server listen: %w", err)
	}
	return nil
}

// Shutdown gracefully stops the listener.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

var _ Shutdowner = (*Server)(nil)
