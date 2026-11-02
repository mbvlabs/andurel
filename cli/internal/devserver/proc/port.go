package proc

import (
	"context"
	"fmt"
	"net"
	"time"
)

const (
	DefaultPortWait  = 5 * time.Second
	portPollInterval = 20 * time.Millisecond
)

// WaitForPortRelease polls until addr can be bound with net.Listen, or returns
// an error when the timeout elapses while the address is still in use.
func WaitForPortRelease(ctx context.Context, addr string, timeout time.Duration) error {
	if addr == "" {
		return nil
	}
	if timeout <= 0 {
		timeout = DefaultPortWait
	}

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(portPollInterval)
	defer ticker.Stop()

	for {
		if ListenAvailable(addr) {
			return nil
		}
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("port %s still in use after waiting %s", addr, timeout)
		case <-ticker.C:
		}
	}
}

// ListenAvailable reports whether a TCP listen on addr succeeds.
func ListenAvailable(addr string) bool {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}
