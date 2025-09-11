package cookies

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
)

type FlashMessage struct {
	echo.Context
	ID        uuid.UUID
	Type      FlashType
	CreatedAt time.Time
	Message   string
}

var (
	FlashKey     flashKey = "flash_key"
	flashSession          = "flash_session"
)

type flashKey string

type FlashType string

const (
	FlashSuccess FlashType = "success"
	FlashError   FlashType = "error"
	FlashWarning FlashType = "warning"
	FlashInfo    FlashType = "info"
)

func AddFlash(
	c echo.Context, flashType FlashType, msg string,
) error {
	sess, err := session.Get(string(FlashKey), c)
	if err != nil {
		return err
	}

	sess.AddFlash(FlashMessage{
		ID:        uuid.New(),
		Type:      flashType,
		CreatedAt: time.Now(),
		Message:   msg,
	}, flashSession)

	return sess.Save(c.Request(), c.Response())
}

func GetFlashes(c echo.Context) ([]FlashMessage, error) {
	sess, err := session.Get(string(FlashKey), c)
	if err != nil {
		return nil, err
	}

	var flashMessages []FlashMessage
	for _, flash := range sess.Flashes(flashSession) {
		if msg, ok := flash.(FlashMessage); ok {
			flashMessages = append(flashMessages, msg)
		}
	}

	if err := sess.Save(c.Request(), c.Response()); err != nil {
		return nil, err
	}

	return flashMessages, nil
}

func GetFlashesCtx(ctx context.Context) []FlashMessage {
	value, ok := ctx.Value(FlashKey).([]FlashMessage)
	if !ok {
		return nil
	}

	return value
}
