package cookies

import (
	"strings"
	"time"

	"github.com/mbvlabs/andurel/pkg/server"

	"github.com/labstack/echo/v5"
	"github.com/rs/xid"
)

type FlashMessage struct {
	ID        xid.ID
	Type      FlashType
	CreatedAt time.Time
	Message   string
}

func buildFlashSessionName(projectName, environment string) string {
	if environment == server.ProdEnvironment {
		return strings.ToLower(projectName) + "_" + "flash_key"
	}

	return strings.ToLower(projectName) + "_" + "dev_flash_key"
}

const flashSessionName = "flash_session"

type FlashType string

const (
	FlashSuccess FlashType = "success"
	FlashError   FlashType = "error"
	FlashWarning FlashType = "warning"
	FlashInfo    FlashType = "info"
)

func (s *Session) AddFlash(
	c *echo.Context, flashType FlashType, msg string,
) error {
	sess, err := getSession(s.flashSessionName, c)
	if err != nil {
		return err
	}

	sess.AddFlash(FlashMessage{
		ID:        xid.New(),
		Type:      flashType,
		CreatedAt: time.Now(),
		Message:   msg,
	}, flashSessionName)

	return sess.Save(c.Request(), c.Response())
}

func (s *Session) ExtractFlashes(c *echo.Context) ([]FlashMessage, error) {
	sess, err := getSession(s.flashSessionName, c)
	if err != nil {
		return nil, err
	}

	var flashMessages []FlashMessage
	for _, flash := range sess.Flashes(flashSessionName) {
		if msg, ok := flash.(FlashMessage); ok {
			flashMessages = append(flashMessages, msg)
		}
	}

	if err := sess.Save(c.Request(), c.Response()); err != nil {
		return nil, err
	}

	return flashMessages, nil
}

// Reflash restores flashes consumed for the current request when Inertia
// returns a redirect, so the next request can still render them.
func (s *Session) Reflash(c *echo.Context, flashes []FlashMessage) error {
	if len(flashes) == 0 {
		return nil
	}
	sess, err := getSession(s.flashSessionName, c)
	if err != nil {
		return err
	}
	for _, flash := range flashes {
		sess.AddFlash(flash, flashSessionName)
	}

	return sess.Save(c.Request(), c.Response())
}
