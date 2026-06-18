package controllers

import (
	"strings"
	"testing"
)

func TestFilterControllerActionsKeepsOnlyRequestedCRUDMethods(t *testing.T) {
	source := `package controllers

import "github.com/labstack/echo/v5"

type Products struct{}

func NewProducts() Products { return Products{} }

func (p Products) Index(etx *echo.Context) error { return nil }

func (p Products) Show(etx *echo.Context) error { return nil }

func (p Products) New(etx *echo.Context) error { return nil }

type CreateProductFormPayload struct{}

func (p Products) Create(etx *echo.Context) error { return nil }

func (p Products) Edit(etx *echo.Context) error { return nil }

type UpdateProductFormPayload struct{}

func (p Products) Update(etx *echo.Context) error { return nil }

func (p Products) Destroy(etx *echo.Context) error { return nil }
`

	filtered, err := filterControllerActions(source, []string{"index", "new"})
	if err != nil {
		t.Fatalf("filterControllerActions returned error: %v", err)
	}

	expectedParts := []string{
		"func NewProducts() Products",
		"func (p Products) Index(",
		"func (p Products) New(",
	}
	for _, part := range expectedParts {
		if !strings.Contains(filtered, part) {
			t.Fatalf("expected filtered source to contain %q:\n%s", part, filtered)
		}
	}

	unexpectedParts := []string{
		"func (p Products) Show(",
		"func (p Products) Create(",
		"func (p Products) Edit(",
		"func (p Products) Update(",
		"func (p Products) Destroy(",
		"type CreateProductFormPayload",
		"type UpdateProductFormPayload",
	}
	for _, part := range unexpectedParts {
		if strings.Contains(filtered, part) {
			t.Fatalf("expected filtered source not to contain %q:\n%s", part, filtered)
		}
	}
}

func TestFilterControllerActionsDoesNotKeepNewUnlessRequested(t *testing.T) {
	source := `package controllers

import "github.com/labstack/echo/v5"

type Products struct{}

func NewProducts() Products { return Products{} }

func (p Products) Index(etx *echo.Context) error { return nil }

func (p Products) Show(etx *echo.Context) error { return nil }

func (p Products) New(etx *echo.Context) error { return nil }
`

	filtered, err := filterControllerActions(source, []string{"index", "show"})
	if err != nil {
		t.Fatalf("filterControllerActions returned error: %v", err)
	}

	expectedParts := []string{
		"func NewProducts() Products",
		"func (p Products) Index(",
		"func (p Products) Show(",
	}
	for _, part := range expectedParts {
		if !strings.Contains(filtered, part) {
			t.Fatalf("expected filtered source to contain %q:\n%s", part, filtered)
		}
	}

	if strings.Contains(filtered, "func (p Products) New(") {
		t.Fatalf("expected filtered source not to contain New action:\n%s", filtered)
	}
}
