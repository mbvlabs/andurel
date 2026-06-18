package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/layout"
	"github.com/mbvlabs/andurel/pkg/constants"
	"github.com/mbvlabs/andurel/pkg/naming"
	"github.com/spf13/cobra"
)

func newGenerateControllerCommand() *cobra.Command {
	var (
		skipRoutes bool
	)

	cmd := &cobra.Command{
		Use:     "controller NAME [action action ...]",
		Aliases: []string{"c"},
		Short:   "Generate a new controller",
		Long: `Generates a new controller, views, and routes. Pass the controller name
in CamelCase and a list of actions as arguments.

This generates a controller with stub methods for each action, view
templates with placeholder content, and route entries.`,
		Example: `  andurel generate controller CreditCards open debit credit close

      Generates a CreditCardsController with open, debit, and credit actions.
      Controller: controllers/credit_cards.go
      Views:      views/credit_cards_resource.templ
      Routes:     router/routes/credit_cards.go
                  router/connect_credit_cards_routes.go

  andurel generate controller Users index --skip-routes

      Generates a UsersController with an index action, no route files.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			name := args[0]
			actions := args[1:]

			if err := chdirToProjectRoot(); err != nil {
				return err
			}

			return withGenerateCleanup(func(_ *cobra.Command, _ []string) error {
				return generateControllerWithActions(name, actions, skipRoutes)
			})(cmd, args)
		},
	}

	cmd.Flags().BoolVar(&skipRoutes, "skip-routes", false, "Don't add routes to router")

	return cmd
}

func generateControllerWithActions(name string, actions []string, skipRoutes bool) error {
	tableName := naming.DeriveTableName(name)
	pluralName := tableName
	modulePath, err := readModulePath()
	if err != nil {
		return fmt.Errorf("failed to read module path: %w", err)
	}

	controllerPath := filepath.Join("controllers", tableName+".go")
	if _, err := os.Stat(controllerPath); err == nil {
		return fmt.Errorf("controller file %s already exists", controllerPath)
	}

	if err := generateActionControllerFile(name, tableName, pluralName, modulePath, controllerPath, actions); err != nil {
		return err
	}

	if !skipRoutes {
		if err := generateActionRoutes(name, tableName, modulePath, actions); err != nil {
			return err
		}
	}

	fmt.Printf("Successfully generated controller %s\n", name)
	return nil
}

func readDIModeForProject() string {
	lock, err := layout.ReadLockFile(".")
	if err != nil || lock.ScaffoldConfig == nil || lock.ScaffoldConfig.DIMode == "" {
		return "manual"
	}
	return lock.ScaffoldConfig.DIMode
}

func generateActionControllerFile(name, tableName, pluralName, modulePath, controllerPath string, actions []string) error {
	ts := naming.ToSnakeCase(name)
	receiverName := naming.ToReceiverName(name)
	resourceName := name

	var sb strings.Builder
	sb.WriteString("package controllers\n\n")
	sb.WriteString("import (\n")
	sb.WriteString("\t\"net/http\"\n")
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("\t\"%s/internal/renderer\"\n", modulePath))
	sb.WriteString("\n")
	sb.WriteString("\t\"github.com/a-h/templ\"\n")
	sb.WriteString("\t\"github.com/labstack/echo/v5\"\n")
	sb.WriteString(")\n\n")
	sb.WriteString(fmt.Sprintf("type %s struct {\n", naming.ToPascalCase(pluralName)))
	sb.WriteString("\tdb interface{ Conn() any }\n")
	sb.WriteString("}\n\n")
	sb.WriteString(fmt.Sprintf("func New%s() %s {\n", naming.ToPascalCase(pluralName), naming.ToPascalCase(pluralName)))
	sb.WriteString(fmt.Sprintf("\treturn %s{}\n", naming.ToPascalCase(pluralName)))
	sb.WriteString("}\n\n")

	for _, action := range actions {
		methodName := naming.ToPascalCase(action)
		sb.WriteString(fmt.Sprintf("func (%s %s) %s(etx echo.Context) error {\n", receiverName, naming.ToPascalCase(pluralName), methodName))
		sb.WriteString(fmt.Sprintf("\treturn renderer.Render(etx, %s%s())\n", naming.ToPascalCase(resourceName), methodName))
		sb.WriteString("}\n\n")
	}

	if err := os.MkdirAll("controllers", 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(controllerPath, []byte(sb.String()), constants.FilePermissionPrivate); err != nil {
		return err
	}

	if err := files.FormatGoFile(controllerPath); err != nil {
		return err
	}

	// Generate view file with action components
	if err := generateActionViewFile(name, tableName, modulePath, ts, actions); err != nil {
		return fmt.Errorf("failed to generate view file: %w", err)
	}

	return nil
}

func generateActionViewFile(name, tableName, modulePath, ts string, actions []string) error {
	resourceName := naming.ToPascalCase(name)
	viewPath := filepath.Join("views", tableName+"_resource.templ")
	if _, err := os.Stat(viewPath); err == nil {
		return fmt.Errorf("view file %s already exists", viewPath)
	}

	var sb strings.Builder
	sb.WriteString("package views\n\n")
	sb.WriteString("import (\n")
	sb.WriteString("\t\"net/http\"\n")
	sb.WriteString(fmt.Sprintf("\t\"%s/internal/renderer\"\n", modulePath))
	sb.WriteString(fmt.Sprintf("\t\"%s/router/routes\"\n", modulePath))
	sb.WriteString(")\n\n")

	for _, action := range actions {
		methodName := naming.ToPascalCase(action)
		sb.WriteString(fmt.Sprintf("templ %s%s() {\n", resourceName, methodName))
		sb.WriteString("\t<div class=\"p-6\">\n")
		sb.WriteString(fmt.Sprintf("\t\t<h1 class=\"text-2xl font-semibold\">%s#%s</h1>\n", resourceName, methodName))
		sb.WriteString("\t\t<p class=\"text-sm text-base-content/60 mt-2\">Content for this action has not been implemented yet.</p>\n")
		sb.WriteString("\t</div>\n")
		sb.WriteString("}\n\n")
	}

	if err := os.MkdirAll("views", 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(viewPath, []byte(sb.String()), constants.FilePermissionPrivate); err != nil {
		return err
	}

	return nil
}

func generateActionRoutes(name, tableName, modulePath string, actions []string) error {
	pluralName := tableName
	resourceName := naming.ToPascalCase(name)
	pluralResource := naming.ToPascalCase(pluralName)
	lowerResource := naming.ToLowerCamelCase(name)

	routesDir := "router/routes"
	routesPath := filepath.Join(routesDir, pluralName+".go")
	if _, err := os.Stat(routesPath); err == nil {
		return fmt.Errorf("routes file %s already exists", routesPath)
	}

	var sb strings.Builder
	sb.WriteString("package routes\n\n")
	sb.WriteString(fmt.Sprintf("import (\n\t\"%s/internal/routing\"\n)\n\n", modulePath))
	sb.WriteString(fmt.Sprintf("const %sPrefix = \"/%s\"\n\n", resourceName, pluralName))

	for _, action := range actions {
		routeName := resourceName + naming.ToPascalCase(action)
		actionLower := naming.ToLowerCamelCase(action)
		path := "/" + actionLower
		sb.WriteString(fmt.Sprintf("var %s = routing.NewSimpleRoute(\n", routeName))
		sb.WriteString(fmt.Sprintf("\t\"%s\",\n", path))
		sb.WriteString(fmt.Sprintf("\t\"%s.%s\",\n", pluralName, actionLower))
		sb.WriteString(fmt.Sprintf("\t%sPrefix,\n", resourceName))
		sb.WriteString(")\n\n")
	}

	if err := os.MkdirAll(routesDir, 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(routesPath, []byte(sb.String()), constants.FilePermissionPrivate); err != nil {
		return err
	}

	if err := files.FormatGoFile(routesPath); err != nil {
		return err
	}

	// Generate route registration file (skipped in uberfx mode)
	if diMode := readDIModeForProject(); diMode != "uberfx" {
		connectPath := filepath.Join("router", "connect_"+pluralName+"_routes.go")
		if _, err := os.Stat(connectPath); err == nil {
			return fmt.Errorf("route registration file %s already exists", connectPath)
		}

		var regSb strings.Builder
		regSb.WriteString("package router\n\n")
		regSb.WriteString("import (\n")
		regSb.WriteString("\t\"errors\"\n")
		regSb.WriteString("\t\"net/http\"\n\n")
		regSb.WriteString(fmt.Sprintf("\t\"%s/controllers\"\n", modulePath))
		regSb.WriteString(fmt.Sprintf("\t\"%s/router/routes\"\n", modulePath))
		regSb.WriteString(")\n\n")
		regSb.WriteString(fmt.Sprintf("func (r Router) Register%sRoutes(%s controllers.%s) error {\n", pluralResource, lowerResource, pluralResource))
		regSb.WriteString("\terrs := []error{}\n\n")

		for _, action := range actions {
			routeName := resourceName + naming.ToPascalCase(action)
			actionTitle := naming.ToPascalCase(action)
			method := actionHTTPMethod(action)
			regSb.WriteString(fmt.Sprintf("\t_, err := r.e.AddRoute(echo.Route{\n"))
			regSb.WriteString(fmt.Sprintf("\t\tMethod: %s,\n", method))
			regSb.WriteString(fmt.Sprintf("\t\tPath:   routes.%s.Path(),\n", routeName))
			regSb.WriteString(fmt.Sprintf("\t\tName:   routes.%s.Name(),\n", routeName))
			regSb.WriteString(fmt.Sprintf("\t\tHandler: %s.%s,\n", lowerResource, actionTitle))
			regSb.WriteString("\t})\n")
			regSb.WriteString("\tif err != nil {\n\t\terrs = append(errs, err)\n\t}\n\n")
		}

		regSb.WriteString("\treturn errors.Join(errs...)\n")
		regSb.WriteString("}\n")

		if err := os.MkdirAll("router", 0o755); err != nil {
			return err
		}

		if err := os.WriteFile(connectPath, []byte(regSb.String()), constants.FilePermissionPrivate); err != nil {
			return err
		}

		if err := files.FormatGoFile(connectPath); err != nil {
			return err
		}
	}

	return nil
}

func actionHTTPMethod(action string) string {
	switch action {
	case "create":
		return "http.MethodPost"
	case "update", "edit":
		return "http.MethodPut"
	case "destroy", "delete":
		return "http.MethodDelete"
	default:
		return "http.MethodGet"
	}
}

func readModulePath() (string, error) {
	content, err := os.ReadFile("go.mod")
	if err != nil {
		return "", fmt.Errorf("failed to read go.mod: %w", err)
	}
	lines := strings.SplitSeq(string(content), "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module")), nil
		}
	}
	return "", fmt.Errorf("module declaration not found in go.mod")
}
