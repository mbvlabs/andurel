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

type AdminUsers struct {
	db database.Postgres
}

func newAdminUsers(db database.Postgres) AdminUsers {
	return AdminUsers{db}
}

func (r AdminUsers) Index(c echo.Context) error {
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

	adminUsersList, err := models.PaginateAdminUsers(
		c.Request().Context(),
		r.db.Conn(),
		page,
		perPage,
	)
	if err != nil {
		return render(c, views.InternalError())
	}

	return render(c, views.AdminUserIndex(adminUsersList.AdminUsers))
}

func (r AdminUsers) Show(c echo.Context) error {
	adminUserID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return render(c, views.BadRequest())
	}

	adminUser, err := models.FindAdminUser(c.Request().Context(), r.db.Conn(), adminUserID)
	if err != nil {
		return render(c, views.NotFound())
	}

	return render(c, views.AdminUserShow(adminUser))
}

func (r AdminUsers) New(c echo.Context) error {
	return render(c, views.AdminUserNew())
}

type CreateAdminUserFormPayload struct {
	Email string `form:"email"`
	Name  string `form:"name"`
	Role  string `form:"role"`
}

func (r AdminUsers) Create(c echo.Context) error {
	var payload CreateAdminUserFormPayload
	if err := c.Bind(&payload); err != nil {
		slog.ErrorContext(
			c.Request().Context(),
			"could not parse CreateAdminUserFormPayload",
			"error",
			err,
		)

		return render(c, views.NotFound())
	}

	data := models.CreateAdminUserData{
		Email: payload.Email,
		Name:  payload.Name,
		Role:  payload.Role,
	}

	adminUser, err := models.CreateAdminUser(
		c.Request().Context(),
		r.db.Conn(),
		data,
	)
	if err != nil {
		if flashErr := cookies.AddFlash(c, cookies.FlashError, fmt.Sprintf("Failed to create adminUser: %v", err)); flashErr != nil {
			return flashErr
		}
		return c.Redirect(http.StatusSeeOther, routes.AdminUserNew.Path)
	}

	if flashErr := cookies.AddFlash(c, cookies.FlashSuccess, "AdminUser created successfully"); flashErr != nil {
		return render(c, views.InternalError())
	}

	return c.Redirect(http.StatusSeeOther, routes.AdminUserShow.GetPath(adminUser.ID))
}

func (r AdminUsers) Edit(c echo.Context) error {
	adminUserID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return render(c, views.BadRequest())
	}

	adminUser, err := models.FindAdminUser(c.Request().Context(), r.db.Conn(), adminUserID)
	if err != nil {
		return render(c, views.NotFound())
	}

	return render(c, views.AdminUserEdit(adminUser))
}

type UpdateAdminUserFormPayload struct {
	Email string `form:"email"`
	Name  string `form:"name"`
	Role  string `form:"role"`
}

func (r AdminUsers) Update(c echo.Context) error {
	adminUserID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return render(c, views.BadRequest())
	}

	var payload UpdateAdminUserFormPayload
	if err := c.Bind(&payload); err != nil {
		slog.ErrorContext(
			c.Request().Context(),
			"could not parse UpdateAdminUserFormPayload",
			"error",
			err,
		)

		return render(c, views.NotFound())
	}

	data := models.UpdateAdminUserData{
		ID:    adminUserID,
		Email: payload.Email,
		Name:  payload.Name,
		Role:  payload.Role,
	}

	adminUser, err := models.UpdateAdminUser(
		c.Request().Context(),
		r.db.Conn(),
		data,
	)
	if err != nil {
		if flashErr := cookies.AddFlash(c, cookies.FlashError, fmt.Sprintf("Failed to update adminUser: %v", err)); flashErr != nil {
			return render(c, views.InternalError())
		}
		return c.Redirect(
			http.StatusSeeOther,
			routes.AdminUserEdit.GetPath(adminUserID),
		)
	}

	if flashErr := cookies.AddFlash(c, cookies.FlashSuccess, "AdminUser updated successfully"); flashErr != nil {
		return render(c, views.InternalError())
	}

	return c.Redirect(http.StatusSeeOther, routes.AdminUserShow.GetPath(adminUser.ID))
}

func (r AdminUsers) Destroy(c echo.Context) error {
	adminUserID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return render(c, views.BadRequest())
	}

	err = models.DestroyAdminUser(c.Request().Context(), r.db.Conn(), adminUserID)
	if err != nil {
		if flashErr := cookies.AddFlash(c, cookies.FlashError, fmt.Sprintf("Failed to delete adminUser: %v", err)); flashErr != nil {
			return render(c, views.InternalError())
		}
		return c.Redirect(http.StatusSeeOther, routes.AdminUserIndex.Path)
	}

	if flashErr := cookies.AddFlash(c, cookies.FlashSuccess, "AdminUser destroyed successfully"); flashErr != nil {
		return render(c, views.InternalError())
	}

	return c.Redirect(http.StatusSeeOther, routes.AdminUserIndex.Path)
}
