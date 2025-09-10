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

type Users struct {
	db database.Postgres
}

func newUsers(db database.Postgres) Users {
	return Users{db}
}

func (r Users) Index(c echo.Context) error {
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

	usersList, err := models.PaginateUsers(
		c.Request().Context(),
		r.db.Pool(),
		page,
		perPage,
	)
	if err != nil {
		return render(c, views.InternalError())
	}

	return render(c, views.UserIndex(usersList.Users))
}

func (r Users) Show(c echo.Context) error {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return render(c, views.BadRequest())
	}

	user, err := models.FindUser(c.Request().Context(), r.db.Pool(), userID)
	if err != nil {
		return render(c, views.NotFound())
	}

	return render(c, views.UserShow(user))
}

func (r Users) New(c echo.Context) error {
	return render(c, views.UserNew())
}

type CreateUserFormPayload struct {
	Email    string `form:"email"`
	Name     string `form:"name"`
	Age      int32  `form:"age"`
	IsActive bool   `form:"is_active"`
}

func (r Users) Create(c echo.Context) error {
	var formPayload CreateUserFormPayload
	if err := c.Bind(&formPayload); err != nil {
		slog.ErrorContext(
			c.Request().Context(),
			"could not parse CreateUserFormPayload",
			"error",
			err,
		)

		return render(c, views.NotFound())
	}

	payload := models.CreateUserPayload{
		Email:    formPayload.Email,
		Name:     formPayload.Name,
		Age:      formPayload.Age,
		IsActive: formPayload.IsActive,
	}

	user, err := models.CreateUser(
		c.Request().Context(),
		r.db.Pool(),
		payload,
	)
	if err != nil {
		if flashErr := cookies.AddFlash(c, cookies.FlashError, fmt.Sprintf("Failed to create user: %v", err)); flashErr != nil {
			return flashErr
		}
		return c.Redirect(http.StatusSeeOther, routes.UserNew.Path)
	}

	if flashErr := cookies.AddFlash(c, cookies.FlashSuccess, "User created successfully"); flashErr != nil {
		return render(c, views.InternalError())
	}

	return c.Redirect(http.StatusSeeOther, routes.UserShow.GetPath(user.ID))
}

func (r Users) Edit(c echo.Context) error {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return render(c, views.BadRequest())
	}

	user, err := models.FindUser(c.Request().Context(), r.db.Pool(), userID)
	if err != nil {
		return render(c, views.NotFound())
	}

	return render(c, views.UserEdit(user))
}

type UpdateUserFormPayload struct {
	Email    string `form:"email"`
	Name     string `form:"name"`
	Age      int32  `form:"age"`
	IsActive bool   `form:"is_active"`
}

func (r Users) Update(c echo.Context) error {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return render(c, views.BadRequest())
	}

	var formPayload UpdateUserFormPayload
	if err := c.Bind(&formPayload); err != nil {
		slog.ErrorContext(
			c.Request().Context(),
			"could not parse UpdateUserFormPayload",
			"error",
			err,
		)

		return render(c, views.NotFound())
	}

	payload := models.UpdateUserPayload{
		ID:       userID,
		Email:    formPayload.Email,
		Name:     formPayload.Name,
		Age:      formPayload.Age,
		IsActive: formPayload.IsActive,
	}

	user, err := models.UpdateUser(
		c.Request().Context(),
		r.db.Pool(),
		payload,
	)
	if err != nil {
		if flashErr := cookies.AddFlash(c, cookies.FlashError, fmt.Sprintf("Failed to update user: %v", err)); flashErr != nil {
			return render(c, views.InternalError())
		}
		return c.Redirect(
			http.StatusSeeOther,
			routes.UserEdit.GetPath(userID),
		)
	}

	if flashErr := cookies.AddFlash(c, cookies.FlashSuccess, "User updated successfully"); flashErr != nil {
		return render(c, views.InternalError())
	}

	return c.Redirect(http.StatusSeeOther, routes.UserShow.GetPath(user.ID))
}

func (r Users) Destroy(c echo.Context) error {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return render(c, views.BadRequest())
	}

	err = models.DestroyUser(c.Request().Context(), r.db.Pool(), userID)
	if err != nil {
		if flashErr := cookies.AddFlash(c, cookies.FlashError, fmt.Sprintf("Failed to delete user: %v", err)); flashErr != nil {
			return render(c, views.InternalError())
		}
		return c.Redirect(http.StatusSeeOther, routes.UserIndex.Path)
	}

	if flashErr := cookies.AddFlash(c, cookies.FlashSuccess, "User destroyed successfully"); flashErr != nil {
		return render(c, views.InternalError())
	}

	return c.Redirect(http.StatusSeeOther, routes.UserIndex.Path)
}
