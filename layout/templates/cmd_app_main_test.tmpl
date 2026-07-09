package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestBackgroundLifecycleStopsOnceAndWaitsForExit(t *testing.T) {
	appCtx, cancelApp := context.WithCancel(context.Background())
	defer cancelApp()
	started := make(chan struct{})
	release := make(chan struct{})
	done := startInBackground(appCtx, "test worker", func(ctx context.Context) error {
		if ctx != appCtx {
			t.Errorf("worker context does not match application context")
		}
		close(started)
		<-release
		return nil
	})
	<-started

	var stopCalls atomic.Int32
	stop := func(context.Context) error {
		if stopCalls.Add(1) == 1 {
			close(release)
		}
		return nil
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := stopAndWait(shutdownCtx, stop, done); err != nil {
		t.Fatalf("stopAndWait: %v", err)
	}
	if got := stopCalls.Load(); got != 1 {
		t.Fatalf("stop calls = %d, want 1", got)
	}
	select {
	case <-done:
	default:
		t.Fatal("background worker still running after shutdown")
	}
}
