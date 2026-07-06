package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mbvlabs/andurel/cli/output"
	generatorpkg "github.com/mbvlabs/andurel/generator"
	controllergen "github.com/mbvlabs/andurel/generator/controllers"
	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/layout"
	"github.com/mbvlabs/andurel/pkg/constants"
	"github.com/mbvlabs/andurel/pkg/naming"
	"github.com/spf13/cobra"
)

func newGenerateControllerCommand() *cobra.Command {
	var (
		inertia   bool
		modelName string
		api       bool
		dryRun    bool
		diff      bool
	)

	cmd := &cobra.Command{
		Use:     "controller NAME [action action ...]",
		Aliases: []string{"c"},
		Short:   "Generate a new controller",
		Long: `Generates a new controller, views, and routes. Pass the controller name
in CamelCase and a list of actions as arguments.

When no actions are provided, this generates the full standard CRUD controller,
views, and routes. When one or more standard CRUD actions are provided
(index, show, new, create, edit, update, destroy), only those resource actions
are generated. Partial CRUD views are self-contained and only link to companion
actions that are also present.

Non-CRUD actions are added as empty controller methods, with matching empty
components in views/<name>_resource.templ or Inertia pages, and conventional
GET routes at /<controllers>/<action>.

Use --model-name when the generated controller/resource name should differ from
the existing model it is backed by. Regular controller generation and scaffold
generation keep the existing one-resource-name behavior unless this flag is
provided.

Names may include one lowercase namespace segment, such as admin/Widget.
Namespaced controllers are generated under controllers/admin, use admin.*
route names, and use Admin-prefixed route and view symbols.

Use --api to generate a JSON API controller instead. The controller is placed
under controllers/api and returns echo.JSON responses. No views are generated.
When --api is provided, the namespace is forced to "api" regardless of any
namespace segment in the name, and the default action set excludes new/edit.`,
		Example: `  andurel generate controller CreditCard

      Generates the standard CRUD resource controller, views, and routes.
      Controller: controllers/credit_cards.go
      Views:      views/credit_cards_resource.templ
      Routes:     router/routes/credit_cards.go

  andurel generate controller CreditCard export

      Adds an empty Export method to controllers/credit_cards.go and an empty
      CreditCardExport component to views/credit_cards_resource.templ.
      Also registers GET /credit_cards/export.

  andurel generate controller admin/Widget export

      Adds an AdminWidgetExport action in controllers/admin/widgets.go,
      views/admin_widgets_resource.templ, and admin.widgets.export route.

  andurel generate controller Dashboard --model-name User

      Generates dashboard controller, views, and routes backed by models.User.

  andurel generate controller Users --api

      Generates a JSON API controller at controllers/api/users.go with
      JSON responses for all CRUD actions. No views are generated.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			name := args[0]
			actions := args[1:]

			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}

			return runMutation(cmd, mutationOptions{
				Action:   "generate controller",
				Resource: name,
				RootDir:  rootDir,
				DryRun:   dryRun,
				Diff:     diff,
				Breadcrumbs: []output.Breadcrumb{
					{Command: "andurel routes --json", Description: "Inspect generated route files"},
					{Command: "andurel doctor", Description: "Verify project health"},
				},
				Run: func(rootDir string) error {
					inertiaStr := ""
					if inertia {
						inertiaStr = generatorpkg.ReadInertia()
					}
					return withGenerateCleanup(func(_ *cobra.Command, _ []string) error {
						return generateControllerWithActionsFunc(name, modelName, actions, inertiaStr, api)
					})(cmd, args)
				},
			})
		},
	}

	cmd.Flags().BoolVar(&api, "api", false, "Generate a JSON API controller under controllers/api")
	cmd.Flags().BoolVar(&inertia, "inertia", false, "Generate Inertia views using the adapter configured in andurel.lock")
	cmd.Flags().StringVar(&modelName, "model-name", "", "Use a different model name for model-backed controller generation")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview file changes without applying")
	cmd.Flags().BoolVar(&diff, "diff", false, "Include a text diff preview in structured output")

	return cmd
}

func generateControllerWithActions(name, modelName string, actions []string, inertia string, isAPI bool) error {
	namespace, resourceName, err := naming.ParseNamespacedResource(name)
	if err != nil {
		return err
	}

	if isAPI {
		namespace = "api"
	}

	tableName := naming.DeriveTableName(resourceName)
	pluralName := tableName
	modulePath, err := readModulePath()
	if err != nil {
		return fmt.Errorf("failed to read module path: %w", err)
	}

	controllerPath := filepath.Join("controllers", namespace, tableName+".go")
	if namespace == "" {
		controllerPath = filepath.Join("controllers", tableName+".go")
	}
	crudActions := crudControllerActions(actions)
	customActions := nonCRUDControllerActions(actions)
	shouldGenerateResource := len(actions) == 0 || len(crudActions) > 0
	modelBackedActions := crudActions
	if len(customActions) > 0 {
		modelBackedActions = append(append([]string(nil), crudActions...), customActions...)
	}
	if modelName != "" && !shouldGenerateResource {
		return fmt.Errorf("--model-name requires a CRUD action or full resource generation")
	}

	if shouldGenerateResource {
		if _, err := os.Stat(controllerPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		gen, err := newGenerator()
		if err != nil {
			return err
		}
		if modelName != "" {
			if err := gen.GenerateControllerWithActionsForModel(resourceName, namespace, modelName, "", modelBackedActions, inertia, isAPI); err != nil {
				return err
			}
		} else if err := gen.GenerateControllerWithActions(resourceName, namespace, "", modelBackedActions, inertia, isAPI); err != nil {
			return err
		}
	}

	if len(customActions) > 0 {
		if err := generateActionControllerFile(resourceName, namespace, tableName, pluralName, modulePath, controllerPath, customActions, inertia, isAPI); err != nil {
			return err
		}
		routeGen := controllergen.NewRouteGenerator()
		if err := routeGen.GenerateRoutes(resourceName, namespace, pluralName, "uuid.UUID", customActions); err != nil {
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

func generateActionControllerFile(name, namespace, tableName, pluralName, modulePath, controllerPath string, actions []string, inertia string, isAPI bool) error {
	ts := namespacePrefix(namespace) + tableName
	receiverName := naming.ToReceiverName(name)
	resourceName := name
	controllerName := naming.ToPascalCase(pluralName)
	isInertia := layout.IsSupportedInertiaAdapter(inertia)
	packageName := naming.ControllerPackageName(namespace)

	if err := os.MkdirAll(filepath.Dir(controllerPath), 0o755); err != nil {
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
			if isAPI {
				additions.WriteString(actionControllerMethodAPI(receiverName, controllerName, resourceName, methodName))
			} else if isInertia {
				additions.WriteString(actionControllerMethodInertia(receiverName, controllerName, namespace, resourceName, methodName))
			} else {
				additions.WriteString(actionControllerMethod(receiverName, controllerName, namespace, resourceName, methodName))
			}
		}

		if additions.Len() > 0 {
			contentStr = strings.TrimRight(contentStr, "\n") + "\n\n" + strings.TrimRight(additions.String(), "\n") + "\n"
		}
		contentStr = ensureCustomFXRegisterRoutes(contentStr, receiverName, namespace, resourceName, actions)

		if err := os.WriteFile(controllerPath, []byte(contentStr), constants.FilePermissionPrivate); err != nil {
			return err
		}
	} else if os.IsNotExist(err) {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("package %s\n\n", packageName))
		sb.WriteString("import (\n")
		if isInertia {
			sb.WriteString("\t\"errors\"\n")
			sb.WriteString("\t\"net/http\"\n")
			sb.WriteString(fmt.Sprintf("\t\"%s/router\"\n", modulePath))
			sb.WriteString(fmt.Sprintf("\t\"%s/router/routes\"\n", modulePath))
			sb.WriteString("\n")
			sb.WriteString(fmt.Sprintf("\t\"%s/internal/inertia\"\n", modulePath))
			sb.WriteString("\n")
			sb.WriteString("\t\"github.com/labstack/echo/v5\"\n")
		} else {
			sb.WriteString("\t\"errors\"\n")
			sb.WriteString("\t\"net/http\"\n")
			sb.WriteString(fmt.Sprintf("\t\"%s/router\"\n", modulePath))
			sb.WriteString(fmt.Sprintf("\t\"%s/router/routes\"\n", modulePath))
			sb.WriteString("\n")
			sb.WriteString(fmt.Sprintf("\t\"%s/internal/hypermedia\"\n", modulePath))
			sb.WriteString(fmt.Sprintf("\t\"%s/views\"\n", modulePath))
			sb.WriteString("\n")
			sb.WriteString("\t\"github.com/labstack/echo/v5\"\n")
		}
		sb.WriteString(")\n\n")
		sb.WriteString(fmt.Sprintf("type %s struct{}\n\n", controllerName))
		sb.WriteString(fmt.Sprintf("func New%s() %s {\n", controllerName, controllerName))
		sb.WriteString(fmt.Sprintf("\treturn %s{}\n", controllerName))
		sb.WriteString("}\n\n")

		sb.WriteString(customFXRegisterRoutesMethod(receiverName, controllerName, namespace, resourceName, actions))
		for _, action := range actions {
			methodName := naming.ToPascalCase(action)
			if isAPI {
				sb.WriteString(actionControllerMethodAPI(receiverName, controllerName, resourceName, methodName))
			} else if isInertia {
				sb.WriteString(actionControllerMethodInertia(receiverName, controllerName, namespace, resourceName, methodName))
			} else {
				sb.WriteString(actionControllerMethod(receiverName, controllerName, namespace, resourceName, methodName))
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
	if isAPI {
		// API controllers don't have views
	} else if isInertia {
		if err := generateActionInertiaViewFile(name, namespace, tableName, actions, inertia); err != nil {
			return fmt.Errorf("failed to generate inertia view file: %w", err)
		}
	} else {
		if err := generateActionViewFile(name, namespace, tableName, modulePath, ts, actions); err != nil {
			return fmt.Errorf("failed to generate view file: %w", err)
		}
	}

	return nil
}

func controllerMethodExists(content, methodName string) bool {
	return strings.Contains(content, ") "+methodName+"(etx *echo.Context)") ||
		strings.Contains(content, ") "+methodName+"(etx echo.Context)")
}

func actionControllerMethod(receiverName, controllerName, namespace, resourceName, methodName string) string {
	namespacePascal := naming.ToPascalCase(namespace)
	return fmt.Sprintf("func (%s %s) %s(etx *echo.Context) error {\n\treturn hypermedia.RenderPage(etx, views.%s%s%s())\n}\n\n",
		receiverName,
		controllerName,
		methodName,
		namespacePascal,
		naming.ToPascalCase(resourceName),
		methodName,
	)
}

func actionControllerMethodAPI(receiverName, controllerName, resourceName, methodName string) string {
	return fmt.Sprintf("func (%s %s) %s(etx *echo.Context) error {\n\treturn etx.JSON(http.StatusOK, map[string]any{})\n}\n\n",
		receiverName,
		controllerName,
		methodName,
	)
}

func actionControllerMethodInertia(receiverName, controllerName, namespace, resourceName, methodName string) string {
	pageName := naming.ToPascalCase(resourceName) + "/" + methodName
	if namespace != "" {
		pageName = naming.ToPascalCase(namespace) + "/" + pageName
	}
	return fmt.Sprintf("func (%s %s) %s(etx *echo.Context) error {\n\treturn inertia.Page(etx, \"%s\", inertia.Props{})\n}\n\n",
		receiverName,
		controllerName,
		methodName,
		pageName,
	)
}

func ensureCustomFXRegisterRoutes(content, receiverName, namespace, resourceName string, actions []string) string {
	if !strings.Contains(content, "RegisterRoutes(r *router.Router)") {
		controllerName := naming.ToPascalCase(naming.DeriveTableName(resourceName))
		return strings.TrimRight(content, "\n") + "\n\n" + strings.TrimRight(customFXRegisterRoutesMethod(receiverName, controllerName, namespace, resourceName, actions), "\n") + "\n"
	}

	var additions strings.Builder
	for _, action := range actions {
		methodName := naming.ToPascalCase(action)
		routeRef := fmt.Sprintf("routes.%s%s%s.Path()", naming.ToPascalCase(namespace), resourceName, methodName)
		if strings.Contains(content, routeRef) {
			continue
		}
		additions.WriteString(customFXRouteBlock(receiverName, namespace, resourceName, methodName))
	}
	if additions.Len() == 0 {
		return content
	}

	needle := "\n\treturn errors.Join(errs...)"
	if strings.Contains(content, needle) {
		return strings.Replace(content, needle, "\n"+strings.TrimRight(additions.String(), "\n")+needle, 1)
	}
	return strings.TrimRight(content, "\n") + "\n\n" + strings.TrimRight(additions.String(), "\n") + "\n"
}

func customFXRegisterRoutesMethod(receiverName, controllerName, namespace, resourceName string, actions []string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("func (%s %s) RegisterRoutes(r *router.Router) error {\n", receiverName, controllerName))
	sb.WriteString("\tvar errs []error\n")
	sb.WriteString("\tvar err error\n\n")
	for _, action := range actions {
		methodName := naming.ToPascalCase(action)
		sb.WriteString(customFXRouteBlock(receiverName, namespace, resourceName, methodName))
	}
	sb.WriteString("\treturn errors.Join(errs...)\n")
	sb.WriteString("}\n\n")
	return sb.String()
}

func customFXRouteBlock(receiverName, namespace, resourceName, methodName string) string {
	return fmt.Sprintf("\t_, err = r.AddRoute(echo.Route{\n\t\tMethod:  http.MethodGet,\n\t\tPath:    routes.%s%s.Path(),\n\t\tName:    routes.%s%s.Name(),\n\t\tHandler: %s.%s,\n\t})\n\tif err != nil {\n\t\terrs = append(errs, err)\n\t}\n\n",
		naming.ToPascalCase(namespace)+resourceName,
		methodName,
		naming.ToPascalCase(namespace)+resourceName,
		methodName,
		receiverName,
		methodName,
	)
}

func generateActionViewFile(name, namespace, tableName, modulePath, ts string, actions []string) error {
	resourceName := naming.ToPascalCase(name)
	namespacePascal := naming.ToPascalCase(namespace)
	viewPath := filepath.Join("views", namespacePrefix(namespace)+tableName+"_resource.templ")

	var sb strings.Builder
	if _, err := os.Stat(viewPath); err == nil {
		content, err := os.ReadFile(viewPath)
		if err != nil {
			return err
		}
		contentStr := string(content)
		for _, action := range actions {
			methodName := naming.ToPascalCase(action)
			componentName := namespacePascal + resourceName + methodName
			if strings.Contains(contentStr, "templ "+componentName+"(") {
				continue
			}
			sb.WriteString(actionViewComponent(resourceName, namespacePascal, methodName))
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
		sb.WriteString(actionViewComponent(resourceName, namespacePascal, methodName))
	}

	if err := os.MkdirAll("views", 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(viewPath, []byte(sb.String()), constants.FilePermissionPrivate); err != nil {
		return err
	}

	return nil
}

func actionViewComponent(resourceName, namespacePascal, methodName string) string {
	var sb strings.Builder
	componentName := namespacePascal + resourceName + methodName
	sb.WriteString(fmt.Sprintf("templ %s() {\n", componentName))
	sb.WriteString("\t<div class=\"p-6\">\n")
	sb.WriteString(fmt.Sprintf("\t\t<h1 class=\"text-2xl font-semibold\">%s#%s</h1>\n", resourceName, methodName))
	sb.WriteString("\t\t<p class=\"text-sm text-base-content/60 mt-2\">Content for this action has not been implemented yet.</p>\n")
	sb.WriteString("\t</div>\n")
	sb.WriteString("}\n\n")
	return sb.String()
}

func generateActionInertiaViewFile(name, namespace, tableName string, actions []string, adapter string) error {
	resourceName := naming.ToPascalCase(name)
	pagesDir := filepath.Join("resources", "js", "Pages", naming.ToPascalCase(namespace), resourceName)

	if err := os.MkdirAll(pagesDir, 0o755); err != nil {
		return err
	}

	for _, action := range actions {
		methodName := naming.ToPascalCase(action)
		viewFilePath := filepath.Join(pagesDir, methodName+inertiaActionViewExtension(adapter))

		if _, err := os.Stat(viewFilePath); err == nil {
			continue
		}

		content := actionInertiaViewComponent(adapter, resourceName, methodName)
		if err := os.WriteFile(viewFilePath, []byte(content), constants.FilePermissionPrivate); err != nil {
			return fmt.Errorf("failed to write inertia view file %s: %w", viewFilePath, err)
		}
	}

	fmt.Printf("Successfully generated inertia views at %s\n", pagesDir)
	return nil
}

func inertiaActionViewExtension(adapter string) string {
	if adapter == "react" {
		return ".tsx"
	}
	return ".vue"
}

func actionInertiaViewComponent(adapter, resourceName, methodName string) string {
	if adapter == "react" {
		return fmt.Sprintf(`import { Head } from '@inertiajs/react'

export default function %s() {
  return (
    <>
      <Head title="%s %s" />
      <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
        <h1 className="text-2xl font-bold text-gray-900">%s#%s</h1>
        <p className="mt-2 text-sm text-gray-500">Content for this action has not been implemented yet.</p>
      </div>
    </>
  )
}
`, methodName, resourceName, methodName, resourceName, methodName)
	}

	return fmt.Sprintf(`<script setup lang="ts">
import { Head } from '@inertiajs/vue3'
</script>

<template>
  <Head title="%s %s" />
  <div class="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
    <h1 class="text-2xl font-bold text-gray-900">%s#%s</h1>
    <p class="mt-2 text-sm text-gray-500">Content for this action has not been implemented yet.</p>
  </div>
</template>
`, resourceName, methodName, resourceName, methodName)
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

func namespacePrefix(namespace string) string {
	if namespace == "" {
		return ""
	}
	return namespace + "_"
}
