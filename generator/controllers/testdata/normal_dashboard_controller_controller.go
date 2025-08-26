package controllers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/example/myapp/database"
	"github.com/example/myapp/models"
	"github.com/example/myapp/router/cookies"
	"github.com/example/myapp/router/routes"
	"github.com/example/myapp/views"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type Dashboards struct {
	db database.Postgres
}
