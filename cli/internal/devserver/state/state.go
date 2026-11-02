package state

import "sync"

const (
	IndexTempl   = 0
	IndexGoBuild = 1
)

type Tracker struct {
	mu      sync.Mutex
	errMsgs [2]string
}

func New() *Tracker {
	return &Tracker{}
}

func (t *Tracker) SetError(index int, msg string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.errMsgs[index] = msg
}

func (t *Tracker) HasError() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, msg := range t.errMsgs {
		if msg != "" {
			return true
		}
	}
	return false
}

func (t *Tracker) HasErrorAt(index int) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.errMsgs[index] != ""
}

func (t *Tracker) ErrorAt(index int) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.errMsgs[index]
}
