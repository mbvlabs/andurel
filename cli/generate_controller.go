package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mbvlabs/andurel/generator"
	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/pkg/constants"
	"github.com/mbvlabs/andurel/pkg/naming"
	"github.com/spf13/cobra"
)

func newGenerateControllerCommand() *cobra.Command {
	var (
		skipRoutes bool
		vue        bool
	)

	cmd := &cobra.Command{
		Use:     "controller NAME [action action ...]",
		Aliases: []string{"c"},
		Short:   "Generate a new controller",
		Long: `Generates a new controller, views, and routes. Pass the controller name
in CamelCase and a list of actions as arguments.

When no actions are provided, or any action is one of index, show, new,
create, edit, update, or destroy, this uses the resource controller templates
and generates the standard CRUD controller, views, and routes.

Non-CRUD actions are added as empty controller methods, with matching empty
components in views/<name>_resource.templ. Custom action routes are not
generated yet.`,
		Example: `  andurel generate controller CreditCard

      Generates the standard CRUD resource controller, views, and routes.
      Controller: controllers/credit_cards.go
      Views:      views/credit_cards_resource.templ
      Routes:     router/routes/credit_cards.go
                  router/connect_credit_cards_routes.go

  andurel generate controller CreditCard export

      Adds an empty Export method to controllers/credit_cards.go and an empty
      CreditCardExport component to views/credit_cards_resource.templ.`,
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

			inertia := ""
			if vue {
				inertia = "vue"
			}

			return withGenerateCleanup(func(_ *cobra.Command, _ []string) error {
				return generateControllerWithActions(name, actions, skipRoutes, inertia)
			})(cmd, args)
		},
	}

	cmd.Flags().BoolVar(&skipRoutes, "skip-routes", false, "Deprecated: custom actions do not generate routes")
	cmd.Flags().BoolVar(&vue, "vue", false, "Generate Inertia Vue views instead of Templ views")

	return cmd
}

func generateControllerWithActions(name string, actions []string, skipRoutes bool, inertia string) error {
	_ = skipRoutes

	tableName := naming.DeriveTableName(name)
	pluralName := tableName
	modulePath, err := readModulePath()
	if err != nil {
		return fmt.Errorf("failed to read module path: %w", err)
	}

	controllerPath := filepath.Join("controllers", tableName+".go")
	crudActions := crudControllerActions(actions)
	customActions := nonCRUDControllerActions(actions)
	shouldGenerateResource := len(actions) == 0 || len(crudActions) > 0

	if shouldGenerateResource {
		if _, err := os.Stat(controllerPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		gen, err := generator.New()
		if err != nil {
			return err
		}
		if err := gen.GenerateControllerWithActions(name, "", true, crudActions, inertia); err != nil {
			return err
		}
	}

	if len(customActions) > 0 {
		if err := generateActionControllerFile(name, tableName, pluralName, modulePath, controllerPath, customActions, inertia); err != nil {
			return err
		}
	}

	fmt.Printf("Successfully generated controller %s\n", name)
	return nil
}

func crudControllerActions(actions []string) []string {
	crudActions := make([]string, 0, len(actions))
	for _, action := range actions {
		action = strings.ToLower(action)
		if isCRUDControllerAction(action) && !slices.Contains(crudActions, action) {
			crudActions = append(crudActions, action)
		}
	}
	return crudActions
}

func nonCRUDControllerActions(actions []string) []string {
	customActions := make([]string, 0, len(actions))
	for _, action := range actions {
		if !isCRUDControllerAction(action) {
			customActions = append(customActions, action)
		}
	}
	return customActions
}

func isCRUDControllerAction(action string) bool {
	switch strings.ToLower(action) {
	case "index", "show", "new", "create", "edit", "update", "destroy":
		return true
	default:
		return false
	}
}

func generateActionControllerFile(name, tableName, pluralName, modulePath, controllerPath string, actions []string, inertia string) error {
	ts := naming.ToSnakeCase(name)
	receiverName := naming.ToReceiverName(name)
	resourceName := name
	controllerName := naming.ToPascalCase(pluralName)
	isInertia := inertia == "vue"

	if err := os.MkdirAll("controllers", 0o755); err != nil {
		return err
	}

	if _, err := os.Stat(controllerPath); err == nil {
		content, err := os.ReadFile(controllerPath)
		if err != nil {
			return err
		}
		contentStr := strings.ReplaceAll(string(content), "(etx echo.Context)", "(etx *echo.Context)")

		var additions strings.Builder
		for _, action := range actions {
			methodName := naming.ToPascalCase(action)
			if controllerMethodExists(contentStr, methodName) {
				continue
			}
			if isInertia {
				additions.WriteString(actionControllerMethodInertia(receiverName, controllerName, resourceName, methodName))
			} else {
				additions.WriteString(actionControllerMethod(receiverName, controllerName, resourceName, methodName))
			}
		}

		if additions.Len() > 0 {
			contentStr = strings.TrimRight(contentStr, "\n") + "\n\n" + strings.TrimRight(additions.String(), "\n") + "\n"
		}

		if err := os.WriteFile(controllerPath, []byte(contentStr), constants.FilePermissionPrivate); err != nil {
			return err
		}
	} else if os.IsNotExist(err) {
		var sb strings.Builder
		sb.WriteString("package controllers\n\n")
		sb.WriteString("import (\n")
		sb.WriteString(fmt.Sprintf("\t\"%s/internal/renderer\"\n", modulePath))
		if isInertia {
			sb.WriteString("\n")
			sb.WriteString("\t\"github.com/labstack/echo/v5\"\n")
			sb.WriteString("\tinertia \"github.com/romsar/gonertia/v3\"\n")
		} else {
			sb.WriteString(fmt.Sprintf("\t\"%s/views\"\n", modulePath))
			sb.WriteString("\n")
			sb.WriteString("\t\"github.com/labstack/echo/v5\"\n")
		}
		sb.WriteString(")\n\n")
		sb.WriteString(fmt.Sprintf("type %s struct{}\n\n", controllerName))
		sb.WriteString(fmt.Sprintf("func New%s() %s {\n", controllerName, controllerName))
		sb.WriteString(fmt.Sprintf("\treturn %s{}\n", controllerName))
		sb.WriteString("}\n\n")

		for _, action := range actions {
			methodName := naming.ToPascalCase(action)
			if isInertia {
				sb.WriteString(actionControllerMethodInertia(receiverName, controllerName, resourceName, methodName))
			} else {
				sb.WriteString(actionControllerMethod(receiverName, controllerName, resourceName, methodName))
			}
		}

		if err := os.WriteFile(controllerPath, []byte(sb.String()), constants.FilePermissionPrivate); err != nil {
			return err
		}
	} else {
		return err
	}

	if err := files.FormatGoFile(controllerPath); err != nil {
		return err
	}

	// Generate view file with action components
	if isInertia {
		if err := generateActionVueViewFile(name, tableName, actions); err != nil {
			return fmt.Errorf("failed to generate vue view file: %w", err)
		}
	} else {
		if err := generateActionViewFile(name, tableName, modulePath, ts, actions); err != nil {
			return fmt.Errorf("failed to generate view file: %w", err)
		}
	}

	return nil
}

func controllerMethodExists(content, methodName string) bool {
	return strings.Contains(content, ") "+methodName+"(etx *echo.Context)") ||
		strings.Contains(content, ") "+methodName+"(etx echo.Context)")
}

func actionControllerMethod(receiverName, controllerName, resourceName, methodName string) string {
	return fmt.Sprintf("func (%s %s) %s(etx *echo.Context) error {\n\treturn renderer.Render(etx, views.%s%s())\n}\n\n",
		receiverName,
		controllerName,
		methodName,
		naming.ToPascalCase(resourceName),
		methodName,
	)
}

func actionControllerMethodInertia(receiverName, controllerName, resourceName, methodName string) string {
	return fmt.Sprintf("func (%s %s) %s(etx *echo.Context) error {\n\treturn renderer.Inertia(etx, \"%s/%s\", inertia.Props{})\n}\n\n",
		receiverName,
		controllerName,
		methodName,
		naming.ToPascalCase(resourceName),
		methodName,
	)
}

func generateActionViewFile(name, tableName, modulePath, ts string, actions []string) error {
	resourceName := naming.ToPascalCase(name)
	viewPath := filepath.Join("views", tableName+"_resource.templ")

	var sb strings.Builder
	if _, err := os.Stat(viewPath); err == nil {
		content, err := os.ReadFile(viewPath)
		if err != nil {
			return err
		}
		contentStr := string(content)
		for _, action := range actions {
			methodName := naming.ToPascalCase(action)
			componentName := resourceName + methodName
			if strings.Contains(contentStr, "templ "+componentName+"(") {
				continue
			}
			sb.WriteString(actionViewComponent(resourceName, methodName))
		}
		if sb.Len() == 0 {
			return nil
		}
		contentStr = strings.TrimRight(contentStr, "\n") + "\n\n" + strings.TrimRight(sb.String(), "\n") + "\n"
		return os.WriteFile(viewPath, []byte(contentStr), constants.FilePermissionPrivate)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	sb.WriteString("package views\n\n")

	for _, action := range actions {
		methodName := naming.ToPascalCase(action)
		sb.WriteString(actionViewComponent(resourceName, methodName))
	}

	if err := os.MkdirAll("views", 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(viewPath, []byte(sb.String()), constants.FilePermissionPrivate); err != nil {
		return err
	}

	return nil
}

func actionViewComponent(resourceName, methodName string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("templ %s%s() {\n", resourceName, methodName))
	sb.WriteString("\t<div class=\"p-6\">\n")
	sb.WriteString(fmt.Sprintf("\t\t<h1 class=\"text-2xl font-semibold\">%s#%s</h1>\n", resourceName, methodName))
	sb.WriteString("\t\t<p class=\"text-sm text-base-content/60 mt-2\">Content for this action has not been implemented yet.</p>\n")
	sb.WriteString("\t</div>\n")
	sb.WriteString("}\n\n")
	return sb.String()
}

func generateActionVueViewFile(name, tableName string, actions []string) error {
	resourceName := naming.ToPascalCase(name)
	pagesDir := filepath.Join("resources", "js", "Pages", resourceName)

	if err := os.MkdirAll(pagesDir, 0o755); err != nil {
		return err
	}

	for _, action := range actions {
		methodName := naming.ToPascalCase(action)
		vueFilePath := filepath.Join(pagesDir, methodName+".vue")

		if _, err := os.Stat(vueFilePath); err == nil {
			continue
		}

		var sb strings.Builder
		sb.WriteString("<script setup lang=\"ts\">\n")
		sb.WriteString("import { Head } from '@inertiajs/vue3'\n")
		sb.WriteString("</script>\n\n")
		sb.WriteString("<template>\n")
		sb.WriteString(fmt.Sprintf("  <Head title=\"%s %s\" />\n", resourceName, methodName))
		sb.WriteString("  <div class=\"mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8\">\n")
		sb.WriteString(fmt.Sprintf("    <h1 class=\"text-2xl font-bold text-gray-900\">%s#%s</h1>\n", resourceName, methodName))
		sb.WriteString("    <p class=\"mt-2 text-sm text-gray-500\">Content for this action has not been implemented yet.</p>\n")
		sb.WriteString("  </div>\n")
		sb.WriteString("</template>\n")

		if err := os.WriteFile(vueFilePath, []byte(sb.String()), constants.FilePermissionPrivate); err != nil {
			return fmt.Errorf("failed to write vue view file %s: %w", vueFilePath, err)
		}
	}

	fmt.Printf("Successfully generated vue views at %s\n", pagesDir)
	return nil
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
