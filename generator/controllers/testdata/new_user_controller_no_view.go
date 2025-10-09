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

type NewUsers struct {
	db database.Postgres
}

func newNewUsers(db database.Postgres) NewUsers {
	return NewUsers{db}
}

func (r NewUsers) Index(c echo.Context) error {
	page := int64(1)
	if p := c.QueryParam("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = int64(parsed)
		}
	}

	perPage := int64(25)
	if pp := c.QueryParam("per_page"); pp != "" {
		if parsed, err := strconv.Atoi(pp); err == nil && parsed > 0 &&
			parsed <= 100 {
			perPage = int64(parsed)
		}
	}

	newUsersList, err := models.PaginateNewUsers(
		c.Request().Context(),
		r.db.Conn(),
		page,
		perPage,
	)
	if err != nil {
		return render(c, views.InternalError())
	}

	return c.HTML(http.StatusOK, "newUsers index - no views implemented")
}

func (r NewUsers) Show(c echo.Context) error {
	newUserID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return render(c, views.BadRequest())
	}

	newUser, err := models.FindNewUser(c.Request().Context(), r.db.Conn(), newUserID)
	if err != nil {
		return render(c, views.NotFound())
	}

	return c.HTML(http.StatusOK, "newUser show - no views implemented")
}

func (r NewUsers) New(c echo.Context) error {
	return c.HTML(http.StatusOK, "newUser new - no views implemented")
}

type CreateNewUserFormPayload struct {
	Email    string `form:"email"`
	Password string `form:"password"`
}

func (r NewUsers) Create(c echo.Context) error {
	var payload CreateNewUserFormPayload
	if err := c.Bind(&payload); err != nil {
		slog.ErrorContext(
			c.Request().Context(),
			"could not parse CreateNewUserFormPayload",
			"error",
			err,
		)

		return render(c, views.NotFound())
	}

	data := models.CreateNewUserData{
		Email:    payload.Email,
		Password: payload.Password,
	}

	newUser, err := models.CreateNewUser(
		c.Request().Context(),
		r.db.Conn(),
		data,
	)
	if err != nil {
		if flashErr := cookies.AddFlash(c, cookies.FlashError, fmt.Sprintf("Failed to create newUser: %v", err)); flashErr != nil {
			return flashErr
		}
		return c.Redirect(http.StatusSeeOther, routes.NewUserNew.Path)
	}

	if flashErr := cookies.AddFlash(c, cookies.FlashSuccess, "NewUser created successfully"); flashErr != nil {
		return render(c, views.InternalError())
	}

	return c.Redirect(http.StatusSeeOther, routes.NewUserShow.GetPath(newUser.ID))
}

func (r NewUsers) Edit(c echo.Context) error {
	newUserID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return render(c, views.BadRequest())
	}

	newUser, err := models.FindNewUser(c.Request().Context(), r.db.Conn(), newUserID)
	if err != nil {
		return render(c, views.NotFound())
	}

	return c.HTML(http.StatusOK, "newUser edit - no views implemented")
}

type UpdateNewUserFormPayload struct {
	Email    string `form:"email"`
	Password string `form:"password"`
}

func (r NewUsers) Update(c echo.Context) error {
	newUserID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return render(c, views.BadRequest())
	}

	var payload UpdateNewUserFormPayload
	if err := c.Bind(&payload); err != nil {
		slog.ErrorContext(
			c.Request().Context(),
			"could not parse UpdateNewUserFormPayload",
			"error",
			err,
		)

		return render(c, views.NotFound())
	}

	data := models.UpdateNewUserData{
		ID:       newUserID,
		Email:    payload.Email,
		Password: payload.Password,
	}

	newUser, err := models.UpdateNewUser(
		c.Request().Context(),
		r.db.Conn(),
		data,
	)
	if err != nil {
		if flashErr := cookies.AddFlash(c, cookies.FlashError, fmt.Sprintf("Failed to update newUser: %v", err)); flashErr != nil {
			return render(c, views.InternalError())
		}
		return c.Redirect(
			http.StatusSeeOther,
			routes.NewUserEdit.GetPath(newUserID),
		)
	}

	if flashErr := cookies.AddFlash(c, cookies.FlashSuccess, "NewUser updated successfully"); flashErr != nil {
		return render(c, views.InternalError())
	}

	return c.Redirect(http.StatusSeeOther, routes.NewUserShow.GetPath(newUser.ID))
}

func (r NewUsers) Destroy(c echo.Context) error {
	newUserID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return render(c, views.BadRequest())
	}

	err = models.DestroyNewUser(c.Request().Context(), r.db.Conn(), newUserID)
	if err != nil {
		if flashErr := cookies.AddFlash(c, cookies.FlashError, fmt.Sprintf("Failed to delete newUser: %v", err)); flashErr != nil {
			return render(c, views.InternalError())
		}
		return c.Redirect(http.StatusSeeOther, routes.NewUserIndex.Path)
	}

	if flashErr := cookies.AddFlash(c, cookies.FlashSuccess, "NewUser destroyed successfully"); flashErr != nil {
		return render(c, views.InternalError())
	}

	return c.Redirect(http.StatusSeeOther, routes.NewUserIndex.Path)
}
