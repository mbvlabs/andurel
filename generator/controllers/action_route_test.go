package controllers

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectRouteTypeAndConstructorName(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantType   RouteType
		idType     string
		paramsType string
		wantCtor   string
	}{
		{"simple", "/products", SimpleRoute, "", "", "NewSimpleRoute"},
		{"uuid id", "/products/:id", RouteWithID, "uuid.UUID", "", "NewRouteWithUUIDID"},
		{"serial id", "/products/:id/edit", RouteWithID, "int32", "", "NewRouteWithSerialID"},
		{"bigserial id", "/products/:id/edit", RouteWithID, "int64", "", "NewRouteWithBigSerialID"},
		{"string id", "/products/:id/edit", RouteWithID, "string", "", "NewRouteWithStringID"},
		{"slug", "/posts/:slug", RouteWithSlug, "", "", "NewRouteWithSlug"},
		{"token", "/invitations/:token", RouteWithToken, "", "", "NewRouteWithToken"},
		{"file", "/downloads/:file", RouteWithFile, "", "", "NewRouteWithFile"},
		{"unknown param as id", "/products/:product_id", RouteWithID, "", "", "NewRouteWithUUIDID"},
		{"multiple ids", "/teams/:team_id/projects/:project_id", RouteWithSlugs, "", "ProjectRouteParams", "NewRouteWithSlugs[ProjectRouteParams]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectRouteType(tt.path)
			if got != tt.wantType {
				t.Fatalf("DetectRouteType(%q) = %v, want %v", tt.path, got, tt.wantType)
			}
			if ctor := got.ConstructorName(tt.idType, tt.paramsType); ctor != tt.wantCtor {
				t.Fatalf("ConstructorName = %q, want %q", ctor, tt.wantCtor)
			}
		})
	}
}

func TestActionInjectorDuplicateAndReadErrors(t *testing.T) {
	injector := NewActionInjector()
	dir := t.TempDir()
	controllerPath := filepath.Join(dir, "products.go")
	routesPath := filepath.Join(dir, "routes.go")

	controller := `package controllers

import "github.com/labstack/echo/v5"

type Products struct{}

func (p Products) Publish(etx *echo.Context) error { return nil }

func (p Products) RegisterRoutes(r any) error {
	// Existing generated registration:
	// Handler: p.Publish,
	return errors.Join(errs...)
}
`
	if err := os.WriteFile(controllerPath, []byte(controller), 0o600); err != nil {
		t.Fatalf("write controller: %v", err)
	}
	if err := os.WriteFile(routesPath, []byte("package routes\n\nvar AdminProductPublish = 1\n"), 0o600); err != nil {
		t.Fatalf("write routes: %v", err)
	}

	if err := injector.InjectControllerMethod(filepath.Join(dir, "missing.go"), ActionMethodData{}); err == nil {
		t.Fatal("expected read error for missing controller")
	}
	if err := injector.InjectControllerMethod(controllerPath, ActionMethodData{MethodName: "Publish"}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected duplicate method error, got %v", err)
	}

	routeData := ActionRouteData{NamespacePascal: "Admin", ResourceName: "Product", MethodName: "Publish"}
	if err := injector.InjectRouteVariable(filepath.Join(dir, "missing_routes.go"), routeData); err == nil {
		t.Fatal("expected read error for missing routes file")
	}
	if err := injector.InjectRouteVariable(routesPath, routeData); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected duplicate route variable error, got %v", err)
	}

	registrationData := ActionRegistrationData{HandlerVar: "p", MethodName: "Publish"}
	if err := injector.InjectRouteRegistration(filepath.Join(dir, "missing_controller.go"), registrationData); err == nil {
		t.Fatal("expected read error for missing controller registration")
	}
	if err := injector.InjectRouteRegistration(controllerPath, registrationData); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected duplicate route registration error, got %v", err)
	}
}

func TestActionInjectorPrintsManualRegistrationWhenReturnIsMissing(t *testing.T) {
	injector := NewActionInjector()
	controllerPath := filepath.Join(t.TempDir(), "products.go")
	original := `package controllers

type Products struct{}

func (p Products) RegisterRoutes(r any) error {
	return nil
}
`
	if err := os.WriteFile(controllerPath, []byte(original), 0o600); err != nil {
		t.Fatalf("write controller: %v", err)
	}

	output := captureStdout(t, func() {
		err := injector.InjectRouteRegistration(controllerPath, ActionRegistrationData{
			ResourceName:    "Product",
			NamespacePascal: "Admin",
			MethodName:      "Publish",
			HTTPMethod:      "Post",
			HandlerVar:      "p",
		})
		if err != nil {
			t.Fatalf("InjectRouteRegistration returned error: %v", err)
		}
	})

	if !strings.Contains(output, "Could not find the RegisterRoutes error return") {
		t.Fatalf("expected manual registration instructions, got:\n%s", output)
	}
	if !strings.Contains(output, "Handler: p.Publish") {
		t.Fatalf("expected handler in manual instructions, got:\n%s", output)
	}

	content, err := os.ReadFile(controllerPath)
	if err != nil {
		t.Fatalf("read controller: %v", err)
	}
	if string(content) != original {
		t.Fatalf("controller without marker should not be modified:\n%s", content)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = old
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close pipe reader: %v", err)
	}
	return buf.String()
}
