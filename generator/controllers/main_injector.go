package controllers

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/pkg/naming"
)

const (
	mainFileRelPath       = "cmd/app/main.go"
	controllerFileRelPath = "controllers/controller.go"
	registrationMarker    = "// andurel:controller-registration-point"
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
func (mi *MainInjector) InjectController(resourceName, namespace, pluralName string) error {
	return mi.InjectControllerWithDB(resourceName, namespace, pluralName, true)
}

func (mi *MainInjector) InjectControllerWithDB(resourceName, namespace, pluralName string, withDB bool) error {
	varName := naming.ToLowerCamelCaseFromAny(pluralName)
	capitalizedPlural := naming.Capitalize(naming.ToCamelCase(pluralName))
	packageName := naming.ControllerPackageName(namespace)
	constructorPackage := packageName
	registrationFunc := "Register" + namespacePrefixPascal(namespace) + resourceName + "Routes"

	// Find go.mod root and construct full path
	rootDir, err := mi.fileManager.FindGoModRoot()
	if err != nil {
		mi.printManualInstructions(resourceName, namespace, pluralName, withDB)
		return nil // Don't fail, just inform
	}

	mainFilePath := filepath.Join(rootDir, mainFileRelPath)

	// Read main.go
	content, err := os.ReadFile(mainFilePath)
	if err != nil {
		mi.printManualInstructions(resourceName, namespace, pluralName, withDB)
		return nil // Don't fail, just inform
	}

	contentStr := string(content)
	if namespace != "" {
		modulePath, err := readModulePathFromRoot(rootDir)
		if err != nil {
			mi.printManualInstructions(resourceName, namespace, pluralName, withDB)
			return nil
		}
		contentStr = ensureImport(contentStr, "", modulePath+"/controllers/"+namespace)
		constructorPackage = namespace
	}

	// Look for marker
	if !strings.Contains(contentStr, registrationMarker) {
		slog.Info("could not find controller registration marker in cmd/app/main.go",
			"marker", registrationMarker,
			"hint", "add the marker to enable automatic controller registration")
		mi.printManualInstructions(resourceName, namespace, pluralName, withDB)
		return nil // Don't fail, just inform
	}

	// Generate injection block
	constructorArgs := ""
	if withDB {
		constructorArgs = "db"
	}
	injection := fmt.Sprintf(`	%s := %s.New%s(%s)
	if err := r.%s(%s); err != nil {
		return err
	}

	`, varName, constructorPackage, capitalizedPlural, constructorArgs, registrationFunc, varName)

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

// InjectFXController adds a generated resource controller to controllers.Module.
// Returns nil if the file or expected module shape is not found, after printing
// instructions for a manual update.
func (mi *MainInjector) InjectFXController(resourceName, namespace, pluralName string) error {
	capitalizedPlural := naming.Capitalize(naming.ToCamelCase(pluralName))
	packageName := naming.ControllerPackageName(namespace)

	rootDir, err := mi.fileManager.FindGoModRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root for fx controller injection: %w", err)
	}

	controllerFilePath := filepath.Join(rootDir, controllerFileRelPath)
	content, err := os.ReadFile(controllerFilePath)
	if err != nil {
		return fmt.Errorf("failed to read controllers/controller.go for fx controller injection: %w", err)
	}

	contentStr := string(content)
	updated := false
	constructorRef := "New" + capitalizedPlural
	constructorProvideRef := constructorRef
	if namespace != "" {
		modulePath, err := readModulePathFromRoot(rootDir)
		if err != nil {
			return fmt.Errorf("failed to read module path for fx controller injection: %w", err)
		}
		nextContent := ensureImport(contentStr, "", modulePath+"/controllers/"+namespace)
		if nextContent != contentStr {
			contentStr = nextContent
			updated = true
		}
		constructorRef = namespace + ".New" + capitalizedPlural
		constructorProvideRef = constructorRef
	}

	nextContent, changed, err := ensureFXConstructor(contentStr, constructorProvideRef)
	if err != nil {
		return err
	}
	if changed {
		contentStr = nextContent
		updated = true
	}

	controllerType := capitalizedPlural
	if namespace != "" {
		controllerType = packageName + "." + capitalizedPlural
	}

	invokeNeedle := fmt.Sprintf("c %s) error", controllerType)
	if !strings.Contains(contentStr, invokeNeedle) {
		invoke := fmt.Sprintf(`	fx.Invoke(func(r *router.Router, c %s) error {
		return c.RegisterRoutes(r)
	}),
`, controllerType)
		nextContent, changed, err := ensureFXModuleEntry(contentStr, invoke)
		if err != nil {
			return err
		}
		if changed {
			contentStr = nextContent
			updated = true
		}
	}

	if !updated {
		return nil
	}

	if err := os.WriteFile(controllerFilePath, []byte(contentStr), 0644); err != nil {
		return fmt.Errorf("failed to write controllers/controller.go: %w", err)
	}

	if err := files.FormatGoFile(controllerFilePath); err != nil {
		return fmt.Errorf("failed to format controllers/controller.go: %w", err)
	}

	return nil
}

func ensureFXConstructor(content, constructorRef string) (string, bool, error) {
	if hasFXReference(content, constructorRef) {
		return content, false, nil
	}

	constructorsIdx := strings.Index(content, "var constructors = fx.Provide(")
	if constructorsIdx != -1 {
		openIdx := strings.Index(content[constructorsIdx:], "(")
		if openIdx == -1 {
			return "", false, fmt.Errorf("failed to locate constructors fx.Provide opening parenthesis")
		}
		openIdx += constructorsIdx
		closeIdx := findMatchingParen(content, openIdx)
		if closeIdx == -1 {
			return "", false, fmt.Errorf("failed to locate constructors fx.Provide closing parenthesis")
		}
		return content[:closeIdx] + "\t" + constructorRef + ",\n" + content[closeIdx:], true, nil
	}

	entry := fmt.Sprintf("\tfx.Provide(%s),\n", constructorRef)
	return ensureFXModuleEntry(content, entry)
}

func hasFXReference(content, ref string) bool {
	refPattern := regexp.QuoteMeta(ref)
	pattern := regexp.MustCompile(`(?m)(^|[[:space:](])` + refPattern + `\s*[,)]`)
	return pattern.FindStringIndex(content) != nil
}

func ensureFXModuleEntry(content, entry string) (string, bool, error) {
	moduleIdx := strings.Index(content, "var Module = fx.Module(")
	if moduleIdx == -1 {
		return "", false, fmt.Errorf("failed to locate controllers fx.Module in controllers/controller.go")
	}
	openIdx := strings.Index(content[moduleIdx:], "(")
	if openIdx == -1 {
		return "", false, fmt.Errorf("failed to locate controllers fx.Module opening parenthesis")
	}
	openIdx += moduleIdx
	closeIdx := findMatchingParen(content, openIdx)
	if closeIdx == -1 {
		return "", false, fmt.Errorf("failed to locate controllers fx.Module closing parenthesis")
	}
	return content[:closeIdx] + entry + content[closeIdx:], true, nil
}

func findMatchingParen(content string, openIdx int) int {
	depth := 0
	for i := openIdx; i < len(content); i++ {
		switch content[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func (mi *MainInjector) printManualInstructions(resourceName, namespace, pluralName string, withDB bool) {
	varName := naming.ToLowerCamelCaseFromAny(pluralName)
	capitalizedPlural := naming.Capitalize(naming.ToCamelCase(pluralName))
	packageName := naming.ControllerPackageName(namespace)
	registrationFunc := "Register" + namespacePrefixPascal(namespace) + resourceName + "Routes"
	constructorArgs := ""
	if withDB {
		constructorArgs = "db"
	}

	fmt.Printf(`
INFO: Add the following to your controller setup in cmd/app/main.go:

	%s := %s.New%s(%s)
	if err := r.%s(%s); err != nil {
		return err
	}

`, varName, packageName, capitalizedPlural, constructorArgs, registrationFunc, varName)
}

func (mi *MainInjector) printFXManualInstructions(resourceName, namespace, pluralName string) {
	capitalizedPlural := naming.Capitalize(naming.ToCamelCase(pluralName))
	packageName := naming.ControllerPackageName(namespace)
	constructorRef := "New" + capitalizedPlural
	controllerType := capitalizedPlural
	if namespace != "" {
		constructorRef = namespace + ".New" + capitalizedPlural
		controllerType = packageName + "." + capitalizedPlural
	}

	fmt.Printf(`
INFO: Add the following to your controller setup in controllers/controller.go:

	%s,

	fx.Invoke(func(r *router.Router, c %s) error {
		return c.RegisterRoutes(r)
	}),

`, constructorRef, controllerType)
}

func namespacePrefixPascal(namespace string) string {
	if namespace == "" {
		return ""
	}
	return naming.ToPascalCase(namespace)
}

func readModulePathFromRoot(rootDir string) (string, error) {
	content, err := os.ReadFile(filepath.Join(rootDir, "go.mod"))
	if err != nil {
		return "", err
	}
	for line := range strings.SplitSeq(string(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", fmt.Errorf("module declaration not found in go.mod")
}

func ensureImport(content, alias, importPath string) string {
	quoted := fmt.Sprintf("%q", importPath)
	if strings.Contains(content, quoted) {
		return content
	}

	importSpec := "\t" + quoted + "\n"
	if alias != "" {
		importSpec = "\t" + alias + " " + quoted + "\n"
	}

	if strings.Contains(content, "import (\n") {
		return strings.Replace(content, "import (\n", "import (\n"+importSpec, 1)
	}
	singleImportRE := regexp.MustCompile(`(?m)^import\s+((?:\w+\s+)?".+")$`)
	if match := singleImportRE.FindStringSubmatchIndex(content); match != nil {
		existing := strings.TrimSpace(content[match[2]:match[3]])
		replacement := "import (\n\t" + existing + "\n" + importSpec + ")"
		return content[:match[0]] + replacement + content[match[1]:]
	}

	lines := strings.SplitN(content, "\n", 2)
	if len(lines) != 2 {
		return content
	}
	return lines[0] + "\n\nimport (\n" + importSpec + ")\n" + lines[1]
}
