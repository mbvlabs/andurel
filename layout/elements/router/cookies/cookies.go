package cookies

import (
	"context"
	"fmt"
	"mbvlabs/andurel/layout/elements/config"
	"strings"

	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
)

var authenticatedSessionName = fmt.Sprintf(
	"ua-%s-%s",
	strings.ToLower(config.App.ProjectName),
	strings.ToLower(config.App.Env),
)

func GetAuthenticatedSessionName() string {
	return authenticatedSessionName
}

const (
	FlashKey         = "flash_messages"
	AppKey           = "app_context"
	isAuthenticated  = "is_authenticated"
	oneWeekInSeconds = 604800
)

type App struct {
	echo.Context
	CurrentPath string
}

func GetAppCtx(ctx context.Context) App {
	appCtx, ok := ctx.Value(AppKey).(App)
	if !ok {
		return App{}
	}

	return appCtx
}

func GetApp(c echo.Context) App {
	_, err := session.Get(authenticatedSessionName, c)
	if err != nil {
		return App{}
	}

	// isAuth, _ := sess.Values[isAuthenticated].(bool)
	// userID, _ := sess.Values[userID].(uuid.UUID)
	// userEmail, _ := sess.Values[userEmail].(string)
	// isAdmin, _ := sess.Values[isAdmin].(bool)

	return App{
		Context: c,
		// UserID:          userID,
		// Email:           userEmail,
		// IsAuthenticated: isAuth,
		// IsAdmin:         isAdmin,
		CurrentPath: c.Request().URL.Path,
	}
}
