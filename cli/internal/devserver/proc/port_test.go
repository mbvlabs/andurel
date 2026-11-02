package proc

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestWaitForPortReleaseSucceedsWhenFree(t *testing.T) {
	addr := freeAddr(t)
	if err := WaitForPortRelease(t.Context(), addr, time.Second); err != nil {
		t.Fatalf("expected free port: %v", err)
	}
}

func TestWaitForPortReleaseErrorsWhenHeld(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	err = WaitForPortRelease(t.Context(), ln.Addr().String(), 80*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error while port held")
	}
}

func TestWaitForPortReleaseWaitsUntilReleased(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()

	go func() {
		time.Sleep(60 * time.Millisecond)
		_ = ln.Close()
	}()

	start := time.Now()
	if err := WaitForPortRelease(context.Background(), addr, time.Second); err != nil {
		t.Fatalf("expected release: %v", err)
	}
	if time.Since(start) < 40*time.Millisecond {
		t.Fatal("returned before holder closed")
	}
}

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}
