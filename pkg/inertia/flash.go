package inertia

import "context"

type flashContextKey struct{}

// ContextWithFlash stores application flash data for the default renderer
// flash provider.
func ContextWithFlash(ctx context.Context, flash any) context.Context {
	return context.WithValue(ctx, flashContextKey{}, flash)
}

// FlashFromContext returns flash data stored by ContextWithFlash.
func FlashFromContext(ctx context.Context) any {
	if ctx == nil {
		return nil
	}
	return ctx.Value(flashContextKey{})
}
