package lifecycle

import (
	"context"
	"errors"
	"os"
	"slices"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestStackClosesInReverseOrderAndJoinsErrors(t *testing.T) {
	var calls []string
	firstErr := errors.New("first close failed")
	secondErr := errors.New("second close failed")
	stack := &Stack{}
	stack.Add(func(context.Context) error {
		calls = append(calls, "first")
		return firstErr
	})
	stack.Add(func(context.Context) error {
		calls = append(calls, "second")
		return secondErr
	})

	err := stack.Close(t.Context())

	if want := []string{"second", "first"}; !slices.Equal(calls, want) {
		t.Fatalf("close order = %v, want %v", calls, want)
	}
	if !errors.Is(err, firstErr) || !errors.Is(err, secondErr) {
		t.Fatalf("Close error = %v, want both close errors", err)
	}
}

func TestNotifyContextCancelsWithParent(t *testing.T) {
	parent, cancelParent := context.WithCancel(t.Context())
	ctx, cancel := NotifyContext(parent)
	defer cancel()

	cancelParent()

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("NotifyContext did not cancel with its parent")
	}
}

func TestNotifyContextCancelsOnSignal(t *testing.T) {
	ctx, cancel := NotifyContext(t.Context())
	defer cancel()

	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("NotifyContext did not cancel on SIGTERM")
	}
}

func TestRunReturnsStartErrorAndStops(t *testing.T) {
	startErr := errors.New("start failed")
	stopErr := errors.New("stop failed")
	var stopped atomic.Int32

	err := Run(
		t.Context(),
		func(context.Context) error { return startErr },
		func(context.Context) error {
			stopped.Add(1)
			return stopErr
		},
	)

	if !errors.Is(err, startErr) {
		t.Fatalf("Run error = %v, want start error", err)
	}
	if errors.Is(err, stopErr) {
		t.Fatalf("Run returned the stop error over the start error: %v", err)
	}
	if got := stopped.Load(); got != 1 {
		t.Fatalf("stop called %d times, want 1", got)
	}
}

func TestRunStopsOnContextCancellation(t *testing.T) {
	stopErr := errors.New("stop failed")
	ctx, cancel := context.WithCancel(t.Context())
	stopped := make(chan struct{})
	ready := make(chan struct{})

	go func() {
		<-ready
		cancel()
	}()

	err := Run(
		ctx,
		func(context.Context) error {
			close(ready)
			return nil
		},
		func(context.Context) error {
			close(stopped)
			return stopErr
		},
	)

	if !errors.Is(err, stopErr) {
		t.Fatalf("Run error = %v, want stop error", err)
	}
	select {
	case <-stopped:
	default:
		t.Fatal("stop was not called")
	}
}

func TestRunStopsWithTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var deadlineErr error
	ready := make(chan struct{})
	go func() {
		<-ready
		cancel()
	}()

	err := Run(
		ctx,
		func(context.Context) error {
			close(ready)
			return nil
		},
		func(stopCtx context.Context) error {
			<-stopCtx.Done()
			deadlineErr = stopCtx.Err()
			return deadlineErr
		},
		WithStopTimeout(10*time.Millisecond),
	)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run error = %v, want deadline exceeded", err)
	}
	if !errors.Is(deadlineErr, context.DeadlineExceeded) {
		t.Fatalf("stop context error = %v, want deadline exceeded", deadlineErr)
	}
}

func TestRunWaitsForStartupBeforeStopping(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var active atomic.Bool
	startErr := errors.New("startup interrupted")

	err := Run(ctx, func(ctx context.Context) error {
		cancel()
		<-ctx.Done()
		// Even cancelled startup may still be finishing resource acquisition.
		time.Sleep(10 * time.Millisecond)
		active.Store(true)
		return startErr
	}, func(context.Context) error {
		if !active.Swap(false) {
			t.Error("stop ran before startup finished")
		}
		return nil
	})

	if !errors.Is(err, startErr) {
		t.Fatalf("Run error = %v, want startup error", err)
	}
	if active.Load() {
		t.Fatal("resource still active after Run returned")
	}
}

func TestRunBlockingStartStopsAndWaitsForServe(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stop := make(chan struct{})
	var exited atomic.Bool

	err := Run(ctx, func(context.Context) error {
		cancel()
		<-stop
		exited.Store(true)
		return nil
	}, func(context.Context) error {
		close(stop)
		return nil
	}, WithBlockingStart())

	if err != nil {
		t.Fatalf("Run error = %v", err)
	}
	if !exited.Load() {
		t.Fatal("Run returned before serve exited")
	}
}

func TestRunBlockingStartReturnsOnServeExit(t *testing.T) {
	for _, serveErr := range []error{nil, errors.New("listen failed")} {
		var stopped bool
		err := Run(t.Context(), func(context.Context) error {
			return serveErr
		}, func(context.Context) error {
			stopped = true
			return errors.New("stop failed")
		}, WithBlockingStart())

		if !stopped {
			t.Fatal("stop was not called after serve exited")
		}
		if serveErr != nil && !errors.Is(err, serveErr) {
			t.Fatalf("Run error = %v, want serve error", err)
		}
		if serveErr == nil && (err == nil || err.Error() != "stop failed") {
			t.Fatalf("Run error = %v, want stop error", err)
		}
	}
}

func TestRunBlockingStartBoundsWaitForServe(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	release := make(chan struct{})
	exited := make(chan struct{})
	defer func() {
		close(release)
		<-exited
	}()

	err := Run(ctx, func(context.Context) error {
		defer close(exited)
		cancel()
		<-release
		return nil
	}, func(context.Context) error {
		return nil
	}, WithBlockingStart(), WithStopTimeout(10*time.Millisecond))

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run error = %v, want deadline exceeded", err)
	}
}
