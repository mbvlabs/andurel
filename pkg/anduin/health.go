package anduin

import (
	"context"
	"errors"
	"net/http"
)

// Health answers GET /health with a JSON body, or a 503 when checker fails.
func Health(checker interface{ Health(context.Context) error }) Handler {
	return func(ctx context.Context) error {
		if err := checker.Health(ctx); err != nil {
			return Error(
				ctx,
				http.StatusServiceUnavailable,
				errors.New("database is unavailable"),
			)
		}

		return JSON(ctx, http.StatusOK, "app is healthy and running")
	}
}
