package kiks

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"
)

// FlashType is the visual category of a flash message.
type FlashType string

const (
	FlashSuccess FlashType = "success"
	FlashError   FlashType = "error"
	FlashWarning FlashType = "warning"
	FlashInfo    FlashType = "info"
)

// FlashMessage is the shared flash contract for Templ and Inertia.
type FlashMessage struct {
	ID        string    `json:"id"`
	Type      FlashType `json:"type"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// Flashes returns the flash messages on the request bag. An empty bag yields
// nil so JSON omitempty and Inertia flash providers skip the field.
func Flashes(ctx context.Context) []FlashMessage {
	requestBag := bagFrom(ctx)
	if requestBag == nil || len(requestBag.flashes) == 0 {
		return nil
	}
	out := make([]FlashMessage, len(requestBag.flashes))
	copy(out, requestBag.flashes)
	return out
}

// AddFlash appends a flash to the current request bag. It is visible to
// whatever this response renders. Persistence is decided by middleware and
// always uses the dedicated flash cookie.
func AddFlash(ctx context.Context, flashType FlashType, message string) {
	requestBag := bagFrom(ctx)
	if requestBag == nil {
		return
	}
	requestBag.flashes = append(requestBag.flashes, FlashMessage{
		ID:        newFlashID(),
		Type:      flashType,
		Message:   message,
		CreatedAt: time.Now(),
	})
	requestBag.flashesDirty = true
}

func newFlashID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("150405.000000000")))
	}
	return hex.EncodeToString(bytes[:])
}
