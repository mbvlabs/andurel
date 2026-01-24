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
	mainFileRelPath    = "cmd/app/main.go"
	registrationMarker = "// andurel:controller-registration-point"
)

type MainInjector struct {
	fileManager files.Manager
}

func NewMainInjector() *MainInjector {
	return &MainInjector{
		fileManager: files.NewUnifiedFileManager(),
	}
}

// InjectController adds controller constructor and registration to main.go
// Returns nil if marker not found (logs info message instead of failing)
func (mi *MainInjector) InjectController(resourceName, pluralName string) error {
	varName := naming.ToLowerCamelCaseFromAny(pluralName)
	capitalizedPlural := naming.Capitalize(naming.ToCamelCase(pluralName))

	// Find go.mod root and construct full path
	rootDir, err := mi.fileManager.FindGoModRoot()
	if err != nil {
		mi.printManualInstructions(resourceName, pluralName)
		return nil // Don't fail, just inform
	}

	mainFilePath := filepath.Join(rootDir, mainFileRelPath)

	// Read main.go
	content, err := os.ReadFile(mainFilePath)
	if err != nil {
		mi.printManualInstructions(resourceName, pluralName)
		return nil // Don't fail, just inform
	}

	contentStr := string(content)

	// Look for marker
	if !strings.Contains(contentStr, registrationMarker) {
		slog.Info("could not find controller registration marker in cmd/app/main.go",
			"marker", registrationMarker,
			"hint", "add the marker to enable automatic controller registration")
		mi.printManualInstructions(resourceName, pluralName)
		return nil // Don't fail, just inform
	}

	// Generate injection block
	injection := fmt.Sprintf(`	%s := controllers.New%s(db)
	if err := r.Register%sRoutes(%s); err != nil {
		return err
	}

	`, varName, capitalizedPlural, resourceName, varName)

	// Insert before marker
	newContent := strings.Replace(contentStr, registrationMarker, injection+registrationMarker, 1)

	// Write back
	if err := os.WriteFile(mainFilePath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write main.go: %w", err)
	}

	// Format with goimports
	if err := files.FormatGoFile(mainFilePath); err != nil {
		return fmt.Errorf("failed to format main.go: %w", err)
	}

	return nil
}

func (mi *MainInjector) printManualInstructions(resourceName, pluralName string) {
	varName := naming.ToLowerCamelCaseFromAny(pluralName)
	capitalizedPlural := naming.Capitalize(naming.ToCamelCase(pluralName))

	fmt.Printf(`
INFO: Add the following to your controller setup in cmd/app/main.go:

	%s := controllers.New%s(db)
	if err := r.Register%sRoutes(%s); err != nil {
		return err
	}

`, varName, capitalizedPlural, resourceName, varName)
}
