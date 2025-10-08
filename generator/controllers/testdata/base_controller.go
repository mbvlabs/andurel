package controllers

import (
	"context"
	"net/http"

	"github.com/mbvlabs/andurel/layout/elements/database"
	"github.com/mbvlabs/andurel/layout/elements/router/cookies"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"
	"github.com/maypok86/otter"
)

type Controllers struct {
	Assets Assets
	API    API
	Pages  Pages
}

func New(
	db database.Postgres,
) (Controllers, error) {
	cacheBuilder, err := otter.NewBuilder[string, templ.Component](20)
	if err != nil {
		return Controllers{}, err
	}

	pageCacher, err := cacheBuilder.WithVariableTTL().Build()
	if err != nil {
		return Controllers{}, err
	}

	assets := newAssets()
	api := newAPI(db)
	pages := newPages(db, pageCacher)

	return Controllers{
		assets,
		api,
		pages,
	}, nil
}

func render(ctx echo.Context, t templ.Component) error {
	buf := templ.GetBuffer()
	defer templ.ReleaseBuffer(buf)

	appCtx := ctx.Get(string(cookies.AppKey))
	withAppCtx := context.WithValue(
		ctx.Request().Context(),
		cookies.AppKey,
		appCtx,
	)

	flashCtx := ctx.Get(string(cookies.FlashKey))
	withFlashCtx := context.WithValue(
		withAppCtx,
		cookies.FlashKey,
		flashCtx,
	)

	if err := t.Render(withFlashCtx, buf); err != nil {
		return err
	}

	return ctx.HTML(http.StatusOK, buf.String())
}