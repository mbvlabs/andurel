package ctxrun

import (
	"testing"
	"time"
)

func TestFanoutDeliversToAllDests(t *testing.T) {
	ctx := t.Context()

	src := make(chan struct{}, 1)
	a := make(chan struct{}, 1)
	b := make(chan struct{}, 1)
	go Fanout(ctx, src, a, b)

	src <- struct{}{}

	for _, ch := range []chan struct{}{a, b} {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatal("expected fanout delivery")
		}
	}
}
