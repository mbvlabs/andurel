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
	providesFileRelPath = "cmd/app/controller_provides.go"
)

type MainInjector struct {
	fileManager files.Manager
}

func NewMainInjector() *MainInjector {
	return &MainInjector{
		fileManager: files.NewUnifiedFileManager(),
	}
}

// InjectController appends an fx.Annotate line to controller_provides.go
func (mi *MainInjector) InjectController(resourceName, pluralName string) error {
	capitalizedPlural := naming.Capitalize(naming.ToCamelCase(pluralName))
	provideLine := fmt.Sprintf("\tfx.Annotate(controllers.New%s, fx.As(new(controllers.Controller))),", capitalizedPlural)

	rootDir, err := mi.fileManager.FindGoModRoot()
	if err != nil {
		mi.printManualInstructions(resourceName, capitalizedPlural)
		return nil
	}

	providesPath := filepath.Join(rootDir, providesFileRelPath)

	content, err := os.ReadFile(providesPath)
	if err != nil {
		mi.printManualInstructions(resourceName, capitalizedPlural)
		return nil
	}

	contentStr := string(content)

	// Check if this controller is already registered
	if strings.Contains(contentStr, provideLine) {
		slog.Info("controller already registered in controller_provides.go")
		return nil
	}

	// Append the new provide line before the closing brace of the var block
	insertion := "\n" + provideLine
	lastBrace := strings.LastIndex(contentStr, "\n}")
	if lastBrace < 0 {
		slog.Info("could not find closing brace in controller_provides.go")
		mi.printManualInstructions(resourceName, capitalizedPlural)
		return nil
	}
	newContent := contentStr[:lastBrace] + insertion + contentStr[lastBrace:]

	if err := os.WriteFile(providesPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write controller_provides.go: %w", err)
	}

	if err := files.FormatGoFile(providesPath); err != nil {
		return fmt.Errorf("failed to format controller_provides.go: %w", err)
	}

	return nil
}

func (mi *MainInjector) printManualInstructions(resourceName, capitalizedPlural string) {
	fmt.Printf(`
INFO: Add the following to cmd/app/controller_provides.go:

	fx.Annotate(controllers.New%s, fx.As(new(controllers.Controller))),
`, capitalizedPlural)
}
