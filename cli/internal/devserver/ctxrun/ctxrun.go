package ctxrun

import (
	"context"
	"sync"
)

type Runner struct {
	lock    sync.Mutex
	counter uint64
	cancel  context.CancelFunc
}

func New() *Runner {
	return &Runner{}
}

func (r *Runner) Go(parent context.Context, fn func(ctx context.Context)) {
	r.lock.Lock()
	if r.cancel != nil {
		r.cancel()
	}
	var ctx context.Context
	ctx, r.cancel = context.WithCancel(parent)
	r.counter++
	r.lock.Unlock()

	go fn(ctx)
}

// Fanout copies each signal from src to every dest without blocking producers.
func Fanout(ctx context.Context, src <-chan struct{}, dests ...chan<- struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-src:
			for _, dest := range dests {
				select {
				case dest <- struct{}{}:
				default:
				}
			}
		}
	}
}
