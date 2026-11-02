package reload

import (
	"testing"
	"time"
)

func TestBroadcastNotifiesListeners(t *testing.T) {
	b := NewBroadcaster()
	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	b.Broadcast()

	select {
	case <-ch:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("listener did not receive broadcast")
	}
}

func TestBroadcastDebounce(t *testing.T) {
	b := NewBroadcaster()
	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	b.Broadcast()
	// Drain the first broadcast
	<-ch

	// Second broadcast within debounce window should be suppressed
	b.Broadcast()

	select {
	case <-ch:
		t.Fatal("broadcast within debounce window should have been suppressed")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestBroadcastAfterDebounceWindow(t *testing.T) {
	b := NewBroadcaster()
	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	b.Broadcast()
	<-ch

	// Wait for debounce to expire
	time.Sleep(b.debounceTime + 10*time.Millisecond)

	b.Broadcast()

	select {
	case <-ch:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("broadcast after debounce window should have been delivered")
	}
}

func TestUnsubscribeRemovesListener(t *testing.T) {
	b := NewBroadcaster()
	ch := b.Subscribe()

	if b.ListenerCount() != 1 {
		t.Fatalf("expected 1 listener, got %d", b.ListenerCount())
	}

	b.Unsubscribe(ch)

	if b.ListenerCount() != 0 {
		t.Fatalf("expected 0 listeners, got %d", b.ListenerCount())
	}
}

func TestMultipleListeners(t *testing.T) {
	b := NewBroadcaster()
	ch1 := b.Subscribe()
	ch2 := b.Subscribe()
	defer b.Unsubscribe(ch1)
	defer b.Unsubscribe(ch2)

	b.Broadcast()

	for i, ch := range []chan struct{}{ch1, ch2} {
		select {
		case <-ch:
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("listener %d did not receive broadcast", i+1)
		}
	}
}
