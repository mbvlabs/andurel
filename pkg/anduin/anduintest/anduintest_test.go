package anduintest

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/mbvlabs/andurel/pkg/anduin"
	"github.com/mbvlabs/andurel/pkg/routing"
)

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
