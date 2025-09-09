package cookies

import (
	"context"
	"fmt"
	"strings"

	"mbvlabs/andurel/layout/elements/config"

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

var AppKey appKey = "app_context"

type appKey string

const (
	isAuthenticated = "is_authenticated"
	// oneWeekInSeconds = 604800
)

type App struct {
	echo.Context
	IsAuthenticated bool
	FlashMessages   []FlashMessage
}

func GetAppCtx(ctx context.Context) App {
	appCtx, ok := ctx.Value(AppKey).(App)
	if !ok {
		return App{}
	}

	return appCtx
}

func GetApp(c echo.Context) App {
	sess, err := session.Get(authenticatedSessionName, c)
	if err != nil {
		return App{}
	}
	app := App{Context: c}

	if _, ok := sess.Values[isAuthenticated].(bool); ok {
		app.IsAuthenticated = true
	}

	return app
}
