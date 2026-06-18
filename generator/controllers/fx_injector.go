package controllers

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/pkg/naming"
)

const (
	controllersModuleRelPath = "controllers/controller.go"
	fxProvideMarker          = "// andurel:fx-controller-provide-point"
	fxInvokeMarker           = "// andurel:fx-controller-invoke-point"
)

type FxInjector struct {
	fileManager files.Manager
}

func NewFxInjector() *FxInjector {
	return &FxInjector{
		fileManager: files.NewUnifiedFileManager(),
	}
}

func (fi *FxInjector) InjectController(resourceName, pluralName string) error {
	rootDir, err := fi.fileManager.FindGoModRoot()
	if err != nil {
		fi.printManualInstructions(resourceName, pluralName)
		return nil
	}

	modulePath := filepath.Join(rootDir, controllersModuleRelPath)
	content, err := os.ReadFile(modulePath)
	if err != nil {
		fi.printManualInstructions(resourceName, pluralName)
		return nil
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, fxProvideMarker) || !strings.Contains(contentStr, fxInvokeMarker) {
		slog.Info("could not find fx controller registration markers",
			"provideMarker", fxProvideMarker,
			"invokeMarker", fxInvokeMarker,
			"hint", "add the markers to enable automatic fx controller registration")
		fi.printManualInstructions(resourceName, pluralName)
		return nil
	}

	capitalizedPlural := naming.Capitalize(naming.ToCamelCase(pluralName))
	provide := fmt.Sprintf("New%s,\n\t", capitalizedPlural)
	invoke := fmt.Sprintf(`fx.Invoke(func(r *router.Router, c %s) error {
		return c.RegisterRoutes(r)
	}),
	`, capitalizedPlural)

	newContent := strings.Replace(contentStr, fxProvideMarker, provide+fxProvideMarker, 1)
	newContent = strings.Replace(newContent, fxInvokeMarker, invoke+fxInvokeMarker, 1)

	if err := os.WriteFile(modulePath, []byte(newContent), 0o644); err != nil {
		return fmt.Errorf("failed to write controllers module: %w", err)
	}

	if err := files.FormatGoFile(modulePath); err != nil {
		return fmt.Errorf("failed to format controllers module: %w", err)
	}

	return nil
}

func (fi *FxInjector) printManualInstructions(resourceName, pluralName string) {
	capitalizedPlural := naming.Capitalize(naming.ToCamelCase(pluralName))

	fmt.Printf(`
INFO: Add the following to controllers/controller.go:

	New%s,

	fx.Invoke(func(r *router.Router, c %s) error {
		return c.RegisterRoutes(r)
	}),

`, capitalizedPlural, capitalizedPlural)
}
