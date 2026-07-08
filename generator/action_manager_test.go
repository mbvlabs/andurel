package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestActionManagerValidateConfig(t *testing.T) {
	manager := NewActionManager()
	tests := []struct {
		name    string
		config  ActionConfig
		wantErr string
	}{
		{
			name:    "valid lowercase method",
			config:  ActionConfig{ControllerName: "Webhook", MethodName: "Validate", Path: "/validate", HTTPMethod: "post"},
			wantErr: "",
		},
		{
			name:    "invalid controller case",
			config:  ActionConfig{ControllerName: "webhook", MethodName: "Validate", Path: "/validate", HTTPMethod: "POST"},
			wantErr: "controller name",
		},
		{
			name:    "invalid method case",
			config:  ActionConfig{ControllerName: "Webhook", MethodName: "validate", Path: "/validate", HTTPMethod: "POST"},
			wantErr: "method name",
		},
		{
			name:    "invalid path",
			config:  ActionConfig{ControllerName: "Webhook", MethodName: "Validate", Path: "validate", HTTPMethod: "POST"},
			wantErr: "must start with",
		},
		{
			name:    "invalid HTTP method",
			config:  ActionConfig{ControllerName: "Webhook", MethodName: "Validate", Path: "/validate", HTTPMethod: "OPTIONS"},
			wantErr: "invalid HTTP method",
		},
		{
			name:    "plural controller name",
			config:  ActionConfig{ControllerName: "Webhooks", MethodName: "Validate", Path: "/validate", HTTPMethod: "POST"},
			wantErr: "should be singular",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.validateConfig(tt.config)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateConfig returned error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestActionManagerHelpers(t *testing.T) {
	manager := NewActionManager()
	routesPath := filepath.Join(t.TempDir(), "routes.go")

	if got := manager.detectIDTypeFromRoutes(filepath.Join(t.TempDir(), "missing.go")); got != "uuid.UUID" {
		t.Fatalf("missing routes ID type = %q", got)
	}

	tests := map[string]string{
		"NewRouteWithSerialID":    "int32",
		"NewRouteWithBigSerialID": "int64",
		"NewRouteWithStringID":    "string",
		"NewRouteWithUUIDID":      "uuid.UUID",
	}
	for constructor, want := range tests {
		if err := os.WriteFile(routesPath, []byte("package routes\nvar _ = routing."+constructor+"\n"), 0o644); err != nil {
			t.Fatalf("write routes: %v", err)
		}
		if got := manager.detectIDTypeFromRoutes(routesPath); got != want {
			t.Fatalf("detectIDTypeFromRoutes(%s) = %q, want %q", constructor, got, want)
		}
	}

	params := manager.buildSlugParamsStruct("ProductReviewParams", "/:product_id/reviews/:review_slug")
	for _, want := range []string{
		"type ProductReviewParams struct",
		"ProductId string `slug:\"product_id\"`",
		"ReviewSlug string `slug:\"review_slug\"`",
	} {
		if !strings.Contains(params, want) {
			t.Fatalf("params struct missing %q:\n%s", want, params)
		}
	}

	methods := map[string]string{
		"GET":     "Get",
		"post":    "Post",
		"PUT":     "Put",
		"delete":  "Delete",
		"PATCH":   "Patch",
		"OPTIONS": "Get",
	}
	for input, want := range methods {
		if got := manager.normalizeHTTPMethodName(input); got != want {
			t.Fatalf("normalizeHTTPMethodName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestActionManagerGenerateActionMissingAndDuplicateFiles(t *testing.T) {
	manager := NewActionManager()
	root := t.TempDir()
	withWorkingDir(t, root)

	config := ActionConfig{ControllerName: "Product", MethodName: "Publish", Path: "/publish", HTTPMethod: "POST"}
	err := manager.GenerateAction(config)
	if err == nil || !strings.Contains(err.Error(), "required file controllers/products.go") {
		t.Fatalf("expected missing controller error, got %v", err)
	}

	writeGeneratorTestFile(t, root, "controllers/products.go", `package controllers

import "github.com/labstack/echo/v5"

type Products struct{}

func (p Products) Publish(etx *echo.Context) error { return nil }
`)
	writeGeneratorTestFile(t, root, "router/routes/products.go", `package routes

var ProductPublish = 1
`)
	err = manager.checkDuplicates(
		filepath.Join("controllers", "products.go"),
		filepath.Join("router", "routes", "products.go"),
		config,
		"p",
	)
	if err == nil || !strings.Contains(err.Error(), "method Publish already exists") {
		t.Fatalf("expected duplicate method error, got %v", err)
	}
}

func TestActionManagerGenerateActionInjectsControllerRouteAndRegistration(t *testing.T) {
	manager := NewActionManager()
	root := t.TempDir()
	withWorkingDir(t, root)

	writeGeneratorTestFile(t, root, "controllers/products.go", `package controllers

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"
	"example.com/app/router"
	"example.com/app/router/routes"
)

type Products struct{}

func (p Products) RegisterRoutes(r *router.Router) error {
	var errs []error
	return errors.Join(errs...)
}
`)
	writeGeneratorTestFile(t, root, "router/routes/products.go", `package routes

import "example.com/app/internal/routing"

const ProductPrefix = "/products"
`)

	err := manager.GenerateAction(ActionConfig{
		ControllerName: "Product",
		MethodName:     "Publish",
		Path:           "/publish",
		HTTPMethod:     "POST",
	})
	if err != nil {
		t.Fatalf("GenerateAction: %v", err)
	}

	controller := readGeneratorTestFile(t, root, "controllers/products.go")
	for _, want := range []string{
		"func (p Products) Publish(etx *echo.Context) error",
		"Method:  http.MethodPost",
		"Handler: p.Publish",
	} {
		if !strings.Contains(controller, want) {
			t.Fatalf("controller missing %q:\n%s", want, controller)
		}
	}

	routes := readGeneratorTestFile(t, root, "router/routes/products.go")
	for _, want := range []string{
		"var ProductPublish = routing.NewSimpleRoute(",
		"\"/publish\"",
		"\"products.publish\"",
	} {
		if !strings.Contains(routes, want) {
			t.Fatalf("routes missing %q:\n%s", want, routes)
		}
	}
}

func writeGeneratorTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func readGeneratorTestFile(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func withWorkingDir(t *testing.T, dir string) {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(original)
	})
}
