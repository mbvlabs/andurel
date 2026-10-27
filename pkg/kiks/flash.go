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

// defaultFlashCookieName names the flash cookie of a jar that has no session.
const defaultFlashCookieName = "kiks_flash"

// FlashMessage is the shared flash contract for Templ and Inertia.
type FlashMessage struct {
	ID        string    `json:"id"`
	Type      FlashType `json:"type"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// Flashes returns the flash messages on the request bag as a JSON-ready value.
// It returns an untyped nil when there are none, so a nil result assigned to
// inertia.Page.Flash is omitted from the page JSON instead of encoded as
// "flash": null. Use FlashMessages when the typed slice is needed.
func Flashes(ctx context.Context) any {
	messages := FlashMessages(ctx)
	if len(messages) == 0 {
		return nil
	}
	return messages
}

// FlashMessages returns a copy of the flash messages on the request bag, or
// nil when there are none.
func FlashMessages(ctx context.Context) []FlashMessage {
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
// always uses the dedicated flash cookie. A request without the middleware's
// bag returns ErrNoBag.
func AddFlash(ctx context.Context, flashType FlashType, message string) error {
	requestBag := bagFrom(ctx)
	if requestBag == nil {
		return ErrNoBag
	}
	requestBag.flashes = append(requestBag.flashes, FlashMessage{
		ID:        newFlashID(),
		Type:      flashType,
		Message:   message,
		CreatedAt: time.Now(),
	})
	requestBag.flashesDirty = true
	return nil
}

func newFlashID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("150405.000000000")))
	}
	return hex.EncodeToString(bytes[:])
}
