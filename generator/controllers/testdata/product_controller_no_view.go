package controllers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/example/shop/database"
	"github.com/example/shop/models"
	"github.com/example/shop/router/cookies"
	"github.com/example/shop/router/routes"
	"github.com/example/shop/views"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type Products struct {
	db database.Postgres
}

func newProducts(db database.Postgres) Products {
	return Products{db}
}

func (r Products) Index(c echo.Context) error {
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

	productsList, err := models.PaginateProducts(
		c.Request().Context(),
		r.db.Pool(),
		page,
		perPage,
	)
	if err != nil {
		return render(c, views.InternalError())
	}

	return c.HTML(http.StatusOK, "products index - no views implemented")
}

func (r Products) Show(c echo.Context) error {
	productID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return render(c, views.BadRequest())
	}

	product, err := models.FindProduct(c.Request().Context(), r.db.Pool(), productID)
	if err != nil {
		return render(c, views.NotFound())
	}

	return c.HTML(http.StatusOK, "product show - no views implemented")
}

func (r Products) New(c echo.Context) error {
	return c.HTML(http.StatusOK, "product new - no views implemented")
}

type CreateProductFormPayload struct {
	Name        string  `form:"name"`
	Price       float64 `form:"price"`
	Description string  `form:"description"`
	CategoryId  int32   `form:"category_id"`
	InStock     bool    `form:"in_stock"`
	Metadata    string  `form:"metadata"`
}

func (r Products) Create(c echo.Context) error {
	var formPayload CreateProductFormPayload
	if err := c.Bind(&formPayload); err != nil {
		slog.ErrorContext(
			c.Request().Context(),
			"could not parse CreateProductFormPayload",
			"error",
			err,
		)

		return render(c, views.NotFound())
	}

	payload := models.CreateProductPayload{
		Name:        formPayload.Name,
		Price:       formPayload.Price,
		Description: formPayload.Description,
		CategoryId:  formPayload.CategoryId,
		InStock:     formPayload.InStock,
		Metadata:    formPayload.Metadata,
	}

	product, err := models.CreateProduct(
		c.Request().Context(),
		r.db.Pool(),
		payload,
	)
	if err != nil {
		if flashErr := cookies.AddFlash(c, cookies.FlashError, fmt.Sprintf("Failed to create product: %v", err)); flashErr != nil {
			return flashErr
		}
		return c.Redirect(http.StatusSeeOther, routes.ProductNew.Path)
	}

	if flashErr := cookies.AddFlash(c, cookies.FlashSuccess, "Product created successfully"); flashErr != nil {
		return render(c, views.InternalError())
	}

	return c.Redirect(http.StatusSeeOther, routes.ProductShow.GetPath(product.ID))
}

func (r Products) Edit(c echo.Context) error {
	productID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return render(c, views.BadRequest())
	}

	product, err := models.FindProduct(c.Request().Context(), r.db.Pool(), productID)
	if err != nil {
		return render(c, views.NotFound())
	}

	return c.HTML(http.StatusOK, "product edit - no views implemented")
}

type UpdateProductFormPayload struct {
	Name        string  `form:"name"`
	Price       float64 `form:"price"`
	Description string  `form:"description"`
	CategoryId  int32   `form:"category_id"`
	InStock     bool    `form:"in_stock"`
	Metadata    string  `form:"metadata"`
}

func (r Products) Update(c echo.Context) error {
	productID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return render(c, views.BadRequest())
	}

	var formPayload UpdateProductFormPayload
	if err := c.Bind(&formPayload); err != nil {
		slog.ErrorContext(
			c.Request().Context(),
			"could not parse UpdateProductFormPayload",
			"error",
			err,
		)

		return render(c, views.NotFound())
	}

	payload := models.UpdateProductPayload{
		ID:          productID,
		Name:        formPayload.Name,
		Price:       formPayload.Price,
		Description: formPayload.Description,
		CategoryId:  formPayload.CategoryId,
		InStock:     formPayload.InStock,
		Metadata:    formPayload.Metadata,
	}

	product, err := models.UpdateProduct(
		c.Request().Context(),
		r.db.Pool(),
		payload,
	)
	if err != nil {
		if flashErr := cookies.AddFlash(c, cookies.FlashError, fmt.Sprintf("Failed to update product: %v", err)); flashErr != nil {
			return render(c, views.InternalError())
		}
		return c.Redirect(
			http.StatusSeeOther,
			routes.ProductEdit.GetPath(productID),
		)
	}

	if flashErr := cookies.AddFlash(c, cookies.FlashSuccess, "Product updated successfully"); flashErr != nil {
		return render(c, views.InternalError())
	}

	return c.Redirect(http.StatusSeeOther, routes.ProductShow.GetPath(product.ID))
}

func (r Products) Destroy(c echo.Context) error {
	productID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return render(c, views.BadRequest())
	}

	err = models.DestroyProduct(c.Request().Context(), r.db.Pool(), productID)
	if err != nil {
		if flashErr := cookies.AddFlash(c, cookies.FlashError, fmt.Sprintf("Failed to delete product: %v", err)); flashErr != nil {
			return render(c, views.InternalError())
		}
		return c.Redirect(http.StatusSeeOther, routes.ProductIndex.Path)
	}

	if flashErr := cookies.AddFlash(c, cookies.FlashSuccess, "Product destroyed successfully"); flashErr != nil {
		return render(c, views.InternalError())
	}

	return c.Redirect(http.StatusSeeOther, routes.ProductIndex.Path)
}
