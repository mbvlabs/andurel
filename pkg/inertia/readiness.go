package inertia

import (
	"context"
	"errors"
	"fmt"
)

// SSRStatus is the stable, machine-readable state of the SSR dependency.
type SSRStatus string

const (
	// SSRStatusReady means SSR is required and the configured sidecar answered a
	// bounded health probe.
	SSRStatusReady SSRStatus = "ready"
	// SSRStatusDisabled means the renderer-wide SSR default is off, so SSR health
	// does not gate readiness. Pages that opt in with PageBuilder.SSR() are the
	// application's own readiness concern.
	SSRStatusDisabled SSRStatus = "disabled"
	// SSRStatusCSRFallback means the renderer-wide default is on with fail-fast
	// off: falling back to client rendering is the explicit policy, so a
	// degraded sidecar does not gate readiness.
	SSRStatusCSRFallback SSRStatus = "csr_fallback"
	// SSRStatusUnavailable means SSR is required and the bounded health probe
	// failed.
	SSRStatusUnavailable SSRStatus = "unavailable"
	// SSRStatusUnsupported means the configured transport exposes no health
	// endpoint: the Vite development server, or a custom SSRRenderer that does
	// not implement SSRHealthReporter. Readiness cannot gate on it.
	SSRStatusUnsupported SSRStatus = "unsupported"
)

// SSRReadiness is the SSR dependency's contribution to an application
// readiness probe. Every call probes the configured transport; no state is
// cached, so a sidecar that recovers is ready on the next call.
type SSRReadiness struct {
	Status SSRStatus
	// Err is the probe failure when Status is SSRStatusUnavailable. Error does
	// not include it, so transport URLs stay out of readiness responses.
	Err error
}

// Ready reports whether readiness must answer healthy for the SSR dependency.
// Only SSRStatusUnavailable makes the application unready; a disabled,
// CSR-fallback or unsupported transport is not a failing dependency.
func (readiness SSRReadiness) Ready() bool {
	return readiness.Status != SSRStatusUnavailable
}

func (readiness SSRReadiness) String() string { return string(readiness.Status) }

// ReadinessError reports a required SSR dependency that is not ready. Its
// message is stable and free of transport details; the cause stays available
// through errors.Unwrap for logs.
type ReadinessError struct {
	Status SSRStatus
	Err    error
}

func (err *ReadinessError) Error() string {
	status := err.Status
	if status == "" {
		status = SSRStatusUnavailable
	}

	return fmt.Sprintf("inertia SSR is not ready: %s", status)
}

func (err *ReadinessError) Unwrap() error {
	if err == nil {
		return nil
	}

	return err.Err
}

// SSRHealthReporter is implemented by SSR transports that answer a bounded
// health probe. HTTPRenderer does; the Vite development transport does not,
// because Vite has no /health endpoint.
type SSRHealthReporter interface {
	Health(context.Context) error
}

// Readiness reports whether this renderer can serve an SSR page right now. SSR
// is required when the renderer-wide default is enabled and SSR failures abort
// the response (WithSSRFailFast); the configured transport is then probed with
// a bounded health request on every call, so losing the sidecar after
// readiness makes the application unready and restarting it makes the
// application ready again. Cancellation of ctx is propagated.
func (renderer *Renderer) Readiness(ctx context.Context) SSRReadiness {
	if renderer == nil {
		return SSRReadiness{
			Status: SSRStatusUnavailable,
			Err:    errors.New("inertia: renderer is nil"),
		}
	}

	if !renderer.ssrDefault {
		return SSRReadiness{Status: SSRStatusDisabled}
	}

	if !renderer.ssrFailFast {
		return SSRReadiness{Status: SSRStatusCSRFallback}
	}

	checker, ok := renderer.ssr.(SSRHealthReporter)
	if !ok || checker == nil {
		return SSRReadiness{Status: SSRStatusUnsupported}
	}

	if ctx == nil {
		ctx = context.Background()
	}

	if err := checker.Health(ctx); err != nil {
		return SSRReadiness{Status: SSRStatusUnavailable, Err: err}
	}

	return SSRReadiness{Status: SSRStatusReady}
}

// Health adapts Readiness to `interface{ Health(context.Context) error }` so a
// readiness handler can compose it with the database check. It returns nil
// unless the SSR dependency is required and unavailable.
func (renderer *Renderer) Health(ctx context.Context) error {
	readiness := renderer.Readiness(ctx)
	if readiness.Ready() {
		return nil
	}

	return &ReadinessError{Status: readiness.Status, Err: readiness.Err}
}
