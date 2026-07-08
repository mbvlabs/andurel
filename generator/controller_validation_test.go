package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewControllerValidationContext(t *testing.T) {
	config := &UnifiedConfig{}
	config.Paths.Controllers = "app/controllers"

	ctx := newControllerValidationContext("CompanyAccount", "company_accounts", "admin/api", config)

	if ctx.ResourceName != "CompanyAccount" {
		t.Fatalf("ResourceName = %q", ctx.ResourceName)
	}
	if ctx.TableName != "company_accounts" {
		t.Fatalf("TableName = %q", ctx.TableName)
	}
	if want := filepath.Join("app/controllers", "admin/api", "company_accounts.go"); ctx.ControllerPath != want {
		t.Fatalf("ControllerPath = %q, want %q", ctx.ControllerPath, want)
	}
	if want := filepath.Join("router/routes", "admin_api_company_accounts.go"); ctx.IndividualRoutePath != want {
		t.Fatalf("IndividualRoutePath = %q, want %q", ctx.IndividualRoutePath, want)
	}
	if ctx.ControllerFieldName != "CompanyAccounts" {
		t.Fatalf("ControllerFieldName = %q", ctx.ControllerFieldName)
	}
	if ctx.ControllerVarName != "companyAccounts" {
		t.Fatalf("ControllerVarName = %q", ctx.ControllerVarName)
	}
	if ctx.ControllerConstructor != "companyAccounts := newCompanyAccounts(db)" {
		t.Fatalf("ControllerConstructor = %q", ctx.ControllerConstructor)
	}
	if ctx.ControllerReturnField != "companyAccounts," {
		t.Fatalf("ControllerReturnField = %q", ctx.ControllerReturnField)
	}
}

func TestValidateControllerNotExists(t *testing.T) {
	root := t.TempDir()
	withWorkingDir(t, root)

	config := &UnifiedConfig{}
	config.Paths.Controllers = "controllers"
	ctx := newControllerValidationContext("Product", "products", "", config)

	err := validateControllerNotExists(ctx)
	if err == nil || !strings.Contains(err.Error(), "controller.go file") {
		t.Fatalf("expected missing controller.go error, got %v", err)
	}

	writeGeneratorTestFile(t, root, "controllers/controller.go", `package controllers

type Controllers struct {
}

func New(db DB) Controllers {
	return Controllers{}
}
`)
	if err := validateControllerNotExists(ctx); err != nil {
		t.Fatalf("validateControllerNotExists returned error: %v", err)
	}

	writeGeneratorTestFile(t, root, "router/routes/products.go", "package routes\n")
	err = validateControllerNotExists(ctx)
	if err == nil || !strings.Contains(err.Error(), "routes file") {
		t.Fatalf("expected existing routes file error, got %v", err)
	}

	if err := os.Remove(filepath.Join(root, "router/routes/products.go")); err != nil {
		t.Fatalf("remove routes file: %v", err)
	}
	writeGeneratorTestFile(t, root, "controllers/products.go", "package controllers\n")
	err = validateControllerNotExists(ctx)
	if err == nil || !strings.Contains(err.Error(), "controller file") {
		t.Fatalf("expected existing controller file error, got %v", err)
	}
}

func TestValidateControllerNotRegistered(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name: "struct field",
			body: `type Controllers struct {
	Products Products
}`,
			wantErr: "struct field found",
		},
		{
			name: "constructor call",
			body: `func New(db DB) Controllers {
	products := newProducts(db)
	return Controllers{}
}`,
			wantErr: "constructor call found",
		},
		{
			name: "return field",
			body: `func New(db DB) Controllers {
	return Controllers{
		products,
	}
}`,
			wantErr: "return field found",
		},
		{
			name: "not registered",
			body: `type Controllers struct {
	Orders Orders
}`,
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			withWorkingDir(t, root)
			writeGeneratorTestFile(t, root, "controllers/controller.go", "package controllers\n\n"+tt.body+"\n")

			config := &UnifiedConfig{}
			config.Paths.Controllers = "controllers"
			ctx := newControllerValidationContext("Product", "products", "", config)

			err := validateControllerNotRegistered(ctx)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateControllerNotRegistered returned error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}
