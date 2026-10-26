package anduin

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

// Counter counts a hit inside a fixed window. cache.Store satisfies it.
type Counter interface {
	Increment(ctx context.Context, key string, ttl time.Duration) (int64, error)
}

// RateLimit allows limit requests per client IP per route within window; every
// attempt counts, successful or not. Each route keeps its own bucket, so a
// burst on one endpoint never locks out another. The client IP comes from
// ClientIP, which only honours X-Forwarded-For from trusted proxies. Over the
// limit, onLimited answers with Retry-After set; a nil onLimited returns a
// plain 429 error.
func RateLimit(
	counter Counter,
	limit int,
	window time.Duration,
	onLimited Handler,
) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context) error {
			key := "ratelimit:" + RoutePattern(ctx) + ":" + ClientIP(ctx)
			hits, err := counter.Increment(ctx, key, window)
			if err != nil {
				return err
			}

			if hits <= int64(limit) {
				return next(ctx)
			}

			SetHeader(ctx, "Retry-After", strconv.Itoa(int(window.Seconds())))
			if onLimited == nil {
				return Errorf(http.StatusTooManyRequests, "too many requests")
			}

			return onLimited(ctx)
		}
	}
}
