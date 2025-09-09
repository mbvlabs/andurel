package controllers

import (
	"github.com/mbvlabs/andurel/layout/elements/database"
	"net/http"

	"github.com/labstack/echo/v4"
)

type API struct {
	db database.Postgres
}

func newAPI(db database.Postgres) API {
	return API{db}
}

func (a API) Health(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, "app is healthy and running")
}
