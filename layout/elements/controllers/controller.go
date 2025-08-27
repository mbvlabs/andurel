package controllers

import (
	"context"
	"io"
	"mbvlabs/andurel/layout/elements/database"
	"net/http"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"
	"github.com/maypok86/otter"
)

type Controllers struct {
	Assets Assets
	API    API
	Pages  Pages
}

func renderArgs(ctx echo.Context) (context.Context, io.Writer) {
	return ctx.Request().Context(), ctx.Response().Writer
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
	pages := newPages(db, pageCacher)
	api := newAPI(db)

	return Controllers{
		assets,
		api,
		pages,
	}, nil
}

func redirectHx(w http.ResponseWriter, url string) error {
	w.Header().Set("HX-Redirect", url)
	w.WriteHeader(http.StatusSeeOther)

	return nil
}

func redirect(
	w http.ResponseWriter,
	r *http.Request,
	url string,
) error {
	http.Redirect(w, r, url, http.StatusSeeOther)
	return nil
}
