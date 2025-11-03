package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/pkg/naming"
)

type controllerValidationContext struct {
	ResourceName           string
	TableName              string
	ControllerPath         string
	RoutesFilePath         string
	IndividualRoutePath    string
	ControllerFilePath     string
	ControllerFieldName    string
	ControllerVarName      string
	ControllerConstructor  string
	ControllerReturnField  string
}

func newControllerValidationContext(resourceName, tableName string, config *UnifiedConfig) *controllerValidationContext {
	controllerFieldName := resourceName + "s"
	controllerVarName := naming.ToCamelCase(naming.ToSnakeCase(resourceName)) + "s"
	controllerConstructor := controllerVarName + " := new" + resourceName + "s(db)"
	controllerReturnField := controllerVarName + ","

	return &controllerValidationContext{
		ResourceName:          resourceName,
		TableName:             tableName,
		ControllerPath:        filepath.Join(config.Paths.Controllers, tableName+".go"),
		RoutesFilePath:        filepath.Join(config.Paths.Routes, "routes.go"),
		IndividualRoutePath:   filepath.Join("router/routes", tableName+".go"),
		ControllerFilePath:    filepath.Join(config.Paths.Controllers, "controller.go"),
		ControllerFieldName:   controllerFieldName,
		ControllerVarName:     controllerVarName,
		ControllerConstructor: controllerConstructor,
		ControllerReturnField: controllerReturnField,
	}
}

func validateControllerNotExists(ctx *controllerValidationContext) error {
	if _, err := os.Stat(ctx.RoutesFilePath); os.IsNotExist(err) {
		return fmt.Errorf(
			"routes file %s does not exist. Please ensure your project has a routes.go file before generating controllers",
			ctx.RoutesFilePath,
		)
	}

	if _, err := os.Stat(ctx.IndividualRoutePath); err == nil {
		return fmt.Errorf("routes file %s already exists", ctx.IndividualRoutePath)
	}

	if _, err := os.Stat(ctx.ControllerPath); err == nil {
		return fmt.Errorf("controller file %s already exists", ctx.ControllerPath)
	}

	if _, err := os.Stat(ctx.ControllerFilePath); os.IsNotExist(err) {
		return fmt.Errorf(
			"main controller file %s does not exist. Please ensure your project has a controller.go file before generating controllers",
			ctx.ControllerFilePath,
		)
	}

	return validateControllerNotRegistered(ctx)
}

func validateControllerNotRegistered(ctx *controllerValidationContext) error {
	content, err := os.ReadFile(ctx.ControllerFilePath)
	if err != nil {
		return fmt.Errorf("failed to read controller.go: %w", err)
	}

	contentStr := string(content)
	lines := strings.SplitSeq(contentStr, "\n")

	for line := range lines {
		trimmedLine := strings.TrimSpace(line)

		if strings.HasPrefix(trimmedLine, ctx.ControllerFieldName+" ") &&
			strings.HasSuffix(trimmedLine, " "+ctx.ControllerFieldName) {
			return fmt.Errorf(
				"controller %s is already registered in %s (struct field found)",
				ctx.ResourceName,
				ctx.ControllerFilePath,
			)
		}

		if strings.Contains(trimmedLine, ctx.ControllerConstructor) {
			return fmt.Errorf(
				"controller %s is already registered in %s (constructor call found)",
				ctx.ResourceName,
				ctx.ControllerFilePath,
			)
		}

		if trimmedLine == ctx.ControllerReturnField {
			return fmt.Errorf(
				"controller %s is already registered in %s (return field found)",
				ctx.ResourceName,
				ctx.ControllerFilePath,
			)
		}
	}

	return nil
}
