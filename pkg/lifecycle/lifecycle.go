// Package lifecycle coordinates process startup and shutdown.
package lifecycle

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"
)

// DefaultStopTimeout bounds how long Run waits for stop to return.
const DefaultStopTimeout = 10 * time.Second

// Stack closes resources in the reverse order they were added.
type Stack struct {
	closers []func(context.Context) error
}

// Add registers a release function. Later additions close first.
func (s *Stack) Add(closeFn func(context.Context) error) {
	s.closers = append(s.closers, closeFn)
}

// Close releases every registered resource, joining their errors.
func (s *Stack) Close(ctx context.Context) error {
	var closeErr error
	for _, closeFn := range slices.Backward(s.closers) {
		closeErr = errors.Join(closeErr, closeFn(ctx))
	}

	return closeErr
}

// Option configures Run.
type Option func(*runOptions)

type runOptions struct {
	stopTimeout   time.Duration
	blockingStart bool
}

// WithStopTimeout sets how long Run waits for stop before returning its error.
func WithStopTimeout(timeout time.Duration) Option {
	return func(options *runOptions) {
		options.stopTimeout = timeout
	}
}

// WithBlockingStart runs start concurrently for servers that serve until stop
// is called. Start and stop must support concurrent calls, including stop before
// start begins. Run waits for start to exit within the shutdown timeout.
func WithBlockingStart() Option {
	return func(options *runOptions) {
		options.blockingStart = true
	}
}

// NotifyContext returns a context cancelled on SIGINT or SIGTERM. Calling the
// cancel function releases the signal handler.
func NotifyContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
}

// Run starts process and stops it on SIGINT, SIGTERM, or a start error. Start
// receives a signal-aware context and returns once the process is ready; Run
// then waits for a signal. Startup completes before stop is called, even when
// cancelled. Use WithBlockingStart for a blocking server instead. A start error
// takes precedence over a stop error.
func Run(
	ctx context.Context,
	start func(context.Context) error,
	stop func(context.Context) error,
	opts ...Option,
) error {
	options := runOptions{stopTimeout: DefaultStopTimeout}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}

	ctx, cancel := NotifyContext(ctx)
	defer cancel()

	var (
		startErr error
		serving  chan error
	)
	if options.blockingStart {
		serving = make(chan error, 1)
		go func() { serving <- start(ctx) }()
		select {
		case startErr = <-serving:
			serving = nil
		case <-ctx.Done():
		}
	} else {
		startErr = start(ctx)
		if startErr == nil {
			<-ctx.Done()
		}
	}

	stopCtx, cancelStop := context.WithTimeout(context.Background(), options.stopTimeout)
	defer cancelStop()

	stopErr := stop(stopCtx)
	if serving != nil {
		select {
		case startErr = <-serving:
		case <-stopCtx.Done():
			return errors.Join(stopErr, stopCtx.Err())
		}
	}
	if startErr != nil {
		return startErr
	}
	return stopErr
}
