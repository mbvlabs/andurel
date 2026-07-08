package generator

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/generator/models"
)

type factorySyncRootFileManager struct {
	root string
}

func (fm factorySyncRootFileManager) ReadFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	return string(content), err
}

func (fm factorySyncRootFileManager) FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (fm factorySyncRootFileManager) WriteFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o600)
}

func (fm factorySyncRootFileManager) EnsureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func (fm factorySyncRootFileManager) ValidateFileNotExists(path string) error {
	if fm.FileExists(path) {
		return os.ErrExist
	}
	return nil
}

func (fm factorySyncRootFileManager) ValidateFileExists(path string) error {
	if !fm.FileExists(path) {
		return os.ErrNotExist
	}
	return nil
}

func (fm factorySyncRootFileManager) FindGoModRoot() (string, error) {
	if fm.root == "" {
		return "", errors.New("missing root")
	}
	return fm.root, nil
}

func TestGeneratedRegionRangesAndOffset(t *testing.T) {
	src := `package factories

// before
// BEGIN ANDUREL FACTORY CORE Product
func generatedCore() {}
// END ANDUREL FACTORY CORE Product

func customHelper() {}

// BEGIN ANDUREL FACTORY OPTIONS Product
func generatedOption() {}
// END ANDUREL FACTORY OPTIONS Product
`

	ranges := generatedRegionRanges(src)
	if len(ranges) != 2 {
		t.Fatalf("expected 2 generated ranges, got %d", len(ranges))
	}

	coreOffset := strings.Index(src, "func generatedCore")
	customOffset := strings.Index(src, "func customHelper")
	optionOffset := strings.Index(src, "func generatedOption")

	if !offsetInRanges(coreOffset, ranges) {
		t.Fatal("expected core function offset to be inside generated region")
	}
	if offsetInRanges(customOffset, ranges) {
		t.Fatal("expected custom helper offset to be outside generated regions")
	}
	if !offsetInRanges(optionOffset, ranges) {
		t.Fatal("expected option function offset to be inside generated region")
	}
}

func TestRenderSyncedFactoryFilePreservesCustomOptionsAndDeclarations(t *testing.T) {
	factory := factorySyncGeneratedFactory()
	oldContent := `package factories

import "math"

// BEGIN ANDUREL FACTORY OPTIONS Product
func WithProductsName(value string) ProductOption {
	return func(f *ProductFactory) {
		f.ProductEntity.Name = value
	}
}
// END ANDUREL FACTORY OPTIONS Product

func WithProductsName(value string) ProductOption {
	return func(f *ProductFactory) {
		f.ProductEntity.Name = "custom:" + value
	}
}

func CustomProductScore() int {
	return int(math.Max(1, 2))
}
`

	rendered, err := renderSyncedFactoryFile(factory, oldContent)
	if err != nil {
		t.Fatalf("renderSyncedFactoryFile returned error: %v", err)
	}

	if _, err := parser.ParseFile(token.NewFileSet(), "product_factory.go", rendered, parser.ParseComments); err != nil {
		t.Fatalf("rendered factory should parse: %v\n%s", err, rendered)
	}

	if count := strings.Count(rendered, "func WithProductsName"); count != 1 {
		t.Fatalf("expected custom option to replace generated option, got %d definitions:\n%s", count, rendered)
	}
	if !strings.Contains(rendered, `"math"`) {
		t.Fatalf("expected preserved custom import:\n%s", rendered)
	}
	if !strings.Contains(rendered, "func CustomProductScore() int") {
		t.Fatalf("expected preserved custom helper:\n%s", rendered)
	}
	if !strings.Contains(rendered, "func WithProductsPrice(value int32) ProductOption") {
		t.Fatalf("expected generated option for non-overridden field:\n%s", rendered)
	}
}

func TestCustomFactoryDeclsReturnsParseErrorForInvalidExistingFactory(t *testing.T) {
	_, _, err := customFactoryDecls("package factories\nfunc broken(", factorySyncGeneratedFactory(), map[string]bool{})
	if err == nil {
		t.Fatal("expected parse error for invalid existing factory")
	}
	if !strings.Contains(err.Error(), "parse existing factory") {
		t.Fatalf("expected parse context in error, got %v", err)
	}
}

func TestSyncFactoryReportsMissingDiffAndWritesWhenRequested(t *testing.T) {
	root := t.TempDir()
	modelsDir := filepath.Join(root, "models")
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		t.Fatalf("create models dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/app\n"), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelsDir, "product.go"), []byte(factorySyncProductModelSource()), 0o600); err != nil {
		t.Fatalf("write product model: %v", err)
	}

	manager := factorySyncTestModelManager(root, modelsDir)
	result, err := manager.SyncFactory("Product", FactorySyncOptions{Diff: true})
	if err != nil {
		t.Fatalf("SyncFactory diff returned error: %v", err)
	}

	if !result.Missing {
		t.Fatal("expected missing factory to be reported")
	}
	if result.Stale {
		t.Fatal("missing factory should not also be marked stale")
	}
	if !result.HasDrift() {
		t.Fatal("missing factory should count as drift")
	}
	if !strings.Contains(result.Diff, "+++ updated") || !strings.Contains(result.Diff, "BuildProduct") {
		t.Fatalf("expected unified diff with generated product factory, got:\n%s", result.Diff)
	}

	written, err := manager.SyncFactory("Product", FactorySyncOptions{Sync: true})
	if err != nil {
		t.Fatalf("SyncFactory sync returned error: %v", err)
	}
	if !written.Written {
		t.Fatal("expected sync to write missing factory")
	}
	content, err := os.ReadFile(filepath.Join(root, "models", "factories", "product.go"))
	if err != nil {
		t.Fatalf("read written factory: %v", err)
	}
	if !strings.Contains(string(content), "func BuildProduct") {
		t.Fatalf("expected written factory content, got:\n%s", content)
	}
}

func TestDiscoverFactoryResourceNames(t *testing.T) {
	modelsDir := t.TempDir()
	files := map[string]string{
		"product.go":      "package models\ntype ProductEntity struct{}\n",
		"account.go":      "package models\ntype AccountEntity struct{}\ntype Ignored string\n",
		"product_test.go": "package models\ntype TestOnlyEntity struct{}\n",
		"broken.go":       "package models\ntype BrokenEntity struct {",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(modelsDir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := os.Mkdir(filepath.Join(modelsDir, "nested"), 0o755); err != nil {
		t.Fatalf("create nested dir: %v", err)
	}

	manager := &ModelManager{config: &UnifiedConfig{Paths: PathConfig{Models: modelsDir}}}
	got, err := manager.discoverFactoryResourceNames()
	if err != nil {
		t.Fatalf("discoverFactoryResourceNames returned error: %v", err)
	}

	want := []string{"Account", "Product"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("discoverFactoryResourceNames = %#v, want %#v", got, want)
	}
}

func factorySyncGeneratedFactory() *models.GeneratedFactory {
	return &models.GeneratedFactory{
		ModelName:     "Product",
		EntityName:    "ProductEntity",
		ModulePath:    "example.com/app",
		IDType:        "uuid.UUID",
		IDGoFieldName: "ID",
		Fields: []models.FactoryField{
			{Name: "ID", Type: "uuid.UUID", IsAutoManaged: true, IsID: true},
			{Name: "Name", Type: "string", DefaultValue: "faker.Name()", OptionName: "WithProductsName"},
			{Name: "Price", Type: "int32", DefaultValue: "randomInt(1, 1000, 100)", OptionName: "WithProductsPrice"},
		},
	}
}

func factorySyncTestModelManager(root, modelsDir string) *ModelManager {
	return &ModelManager{
		fileManager:    factorySyncRootFileManager{root: root},
		modelGenerator: models.NewGenerator("postgresql"),
		projectManager: &ProjectManager{modulePath: "example.com/app"},
		config: &UnifiedConfig{
			Database: DatabaseConfig{Type: "postgresql"},
			Paths:    PathConfig{Models: modelsDir},
		},
	}
}

func factorySyncProductModelSource() string {
	return `package models

type ProductEntity struct {
	ID        uuid.UUID ` + "`bun:\"id,pk,type:uuid\"`" + `
	Name      string    ` + "`bun:\"name,notnull\"`" + `
	Price     int32     ` + "`bun:\"price,notnull\"`" + `
	CreatedAt time.Time ` + "`bun:\"created_at,notnull\"`" + `
}
`
}
