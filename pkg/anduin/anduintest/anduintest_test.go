package anduintest

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/routing"
)

func TestWithHostsDispatchesNamedHost(t *testing.T) {
	t.Parallel()

	const hostAdmin routing.HostName = "admin"
	hosts, err := routing.NewHosts(map[routing.HostName]routing.HostSpec{
		routing.HostPrimary: {Hostname: "andurel.example", Protocol: "http"},
		hostAdmin:           {Hostname: "admin.andurel.example"},
	})
	if err != nil {
		t.Fatalf("new hosts: %v", err)
	}

	server := New(t, func(rt *anduin.Router) {
		rt.Get("/", func(ctx context.Context) error {
			return anduin.Text(ctx, http.StatusOK, "primary")
		})
		rt.Host(hostAdmin).Get("/", func(ctx context.Context) error {
			return anduin.Text(ctx, http.StatusOK, "admin")
		})
	}, WithHosts(hosts))

	if got := server.Get("/").Text(); got != "primary" {
		t.Fatalf("default host body = %q, want primary", got)
	}

	admin := server.Get("/", Header{Name: "Host", Value: "admin.andurel.example"})
	admin.AssertStatus(http.StatusOK)
	if got := admin.Text(); got != "admin" {
		t.Fatalf("admin host body = %q, want admin", got)
	}

	upper := server.Get("/", Header{Name: "Host", Value: "ADMIN.Andurel.Example"})
	if got := upper.Text(); got != "admin" {
		t.Fatalf("uppercase admin host body = %q, want admin", got)
	}
}

func TestWithErrorHandlerMapsNotFound(t *testing.T) {
	t.Parallel()

	server := New(t, func(rt *anduin.Router) {
		home := routing.NewSimpleRoute("/", "pages.home", "", routing.Exact())
		rt.Get(home.Pattern(), func(ctx context.Context) error {
			return anduin.Text(ctx, http.StatusOK, "home")
		})
	}, WithErrorHandler(func(ctx context.Context, err error) error {
		if !errors.Is(err, anduin.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
		return anduin.Text(ctx, http.StatusNotFound, "themed not found")
	}))

	server.Get("/").AssertStatus(http.StatusOK)

	missing := server.Get("/missing")
	missing.AssertStatus(http.StatusNotFound)
	if got := missing.Text(); got != "themed not found" {
		t.Fatalf("body = %q, want themed not found", got)
	}
}
