// Package server provides an HTTP server with graceful shutdown capabilities.
// Code generated and maintained by the andurel framework. DO NOT EDIT.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"
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

func New(
	ctx context.Context,
	host string,
	port string,
	env string,
	handler http.Handler,
	shutdowners []Shutdowner,
	options ...ServerOption,
) Server {
	serverOptions := &ServerOptions{
		IdleTimeout:  120 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	// Apply server options if any
	for _, option := range options {
		option(serverOptions)
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
		Shutdowners: []Shutdowner{srv},
	}

	server.Shutdowners = append(server.Shutdowners, shutdowners...)

	return server
}

func (s *Server) Start(
	ctx context.Context,
	env string,
) error {
	if env == ProdEnvironment {
		eg, egCtx := errgroup.WithContext(ctx)

		eg.Go(func() error {
			if err := s.srv.ListenAndServe(); err != nil &&
				err != http.ErrServerClosed {
				return fmt.Errorf("server error: %w", err)
			}
			return nil
		})

		eg.Go(func() error {
			<-egCtx.Done()
			slog.InfoContext(ctx, "initiating graceful shutdown")
			shutdownCtx, cancel := context.WithTimeout(
				ctx,
				10*time.Second,
			)
			defer cancel()

			for _, shutdowner := range s.Shutdowners {
				slog.InfoContext(
					ctx,
					"shutting down component",
					"component",
					fmt.Sprintf("%T", shutdowner),
				)
				if err := shutdowner.Shutdown(shutdownCtx); err != nil {
					return fmt.Errorf("component shutdown error (%T): %w", shutdowner, err)
				}
			}

			return nil
		})

		if err := eg.Wait(); err != nil {
			slog.InfoContext(ctx, "wait error", "e", err)
			return err
		}

		return nil
	}

	return s.srv.ListenAndServe()
}

