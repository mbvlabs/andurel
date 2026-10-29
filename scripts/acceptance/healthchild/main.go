// Command healthchild is a deterministic acceptance fixture for
// `andurel ports launch`: it binds INERTIA_SSR_LISTEN and answers the launcher
// readiness probe with {"status":"ok"} until it receives SIGTERM/SIGINT.
//
// It is not a production SSR renderer. The acceptance harness uses it to prove
// port leases, readiness publication, and clean shutdown without Node.
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	listenURL := os.Getenv("INERTIA_SSR_LISTEN")
	if listenURL == "" {
		fmt.Fprintln(os.Stderr, "healthchild: INERTIA_SSR_LISTEN is required")
		os.Exit(2)
	}
	address := strings.TrimPrefix(strings.TrimPrefix(listenURL, "http://"), "https://")
	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthchild: listen:", err)
		os.Exit(2)
	}

	server := &http.Server{
		Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path != "/health" {
				http.NotFound(writer, request)
				return
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"status":"ok"}`))
		}),
		ReadHeaderTimeout: 2 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	fmt.Fprintln(os.Stderr, "healthchild listening on", listener.Addr().String())
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, "healthchild: serve:", err)
		os.Exit(1)
	}
}
