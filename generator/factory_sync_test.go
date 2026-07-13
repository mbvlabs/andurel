package generator

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
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

func TestRenderSyncedFactoryFileRegeneratesOptionsAndPreservesCustomDeclarations(t *testing.T) {
	factory := factorySyncGeneratedFactory()
	oldContent := `package factories

import (
	"math"
	"strings"
)

func WithProductsName(value string) ProductOption {
	return func(f *ProductFactory) {
		f.ProductEntity.Name = strings.ToUpper(value)
	}
}

func WithProductsPrice(value int16) ProductOption {
	return func(f *ProductFactory) {
		f.ProductEntity.Price = int32(value)
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
		t.Fatalf("expected one regenerated name option, got %d definitions:\n%s", count, rendered)
	}
	if strings.Contains(rendered, `"custom:"`) {
		t.Fatalf("expected same-name custom option to be replaced by generated option:\n%s", rendered)
	}
	if !strings.Contains(rendered, `"math"`) {
		t.Fatalf("expected preserved custom import:\n%s", rendered)
	}
	if strings.Contains(rendered, `"strings"`) {
		t.Fatalf("expected import used only by discarded option override to be removed:\n%s", rendered)
	}
	if !strings.Contains(rendered, "func CustomProductScore() int") {
		t.Fatalf("expected preserved custom helper:\n%s", rendered)
	}
	if !strings.Contains(rendered, "func WithProductsPrice(value int32) ProductOption") {
		t.Fatalf("expected stale same-name option signature to be regenerated:\n%s", rendered)
	}
	if strings.Contains(rendered, "func WithProductsPrice(value int16) ProductOption") {
		t.Fatalf("expected stale same-name option signature to be removed:\n%s", rendered)
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

func TestSyncFactoriesHandlesMultipleModelsAndCurrentFactories(t *testing.T) {
	root := t.TempDir()
	modelsDir := filepath.Join(root, "models")
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		t.Fatalf("create models directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/app\n"), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	for _, resource := range []string{"Product", "Account"} {
		path := filepath.Join(modelsDir, strings.ToLower(resource)+".go")
		source := strings.ReplaceAll(factorySyncProductModelSource(), "ProductEntity", resource+"Entity")
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatalf("write %s model: %v", resource, err)
		}
	}

	manager := factorySyncTestModelManager(root, modelsDir)
	results, err := manager.SyncFactories(FactorySyncOptions{Sync: true, Diff: true})
	if err != nil {
		t.Fatalf("sync factories: %v", err)
	}
	if len(results) != 2 || results[0].ResourceName != "Account" || results[1].ResourceName != "Product" {
		t.Fatalf("unexpected sorted results: %#v", results)
	}
	for _, result := range results {
		if !result.Missing || !result.Written || !result.HasDrift() || result.Diff == "" {
			t.Fatalf("unexpected initial sync result: %#v", result)
		}
	}

	current, err := manager.SyncFactories(FactorySyncOptions{Check: true, Diff: true})
	if err != nil {
		t.Fatalf("check current factories: %v", err)
	}
	for _, result := range current {
		if result.HasDrift() || result.Written || result.Diff != "" {
			t.Fatalf("current factory reported drift: %#v", result)
		}
	}

	if _, err := manager.SyncFactory("Missing", FactorySyncOptions{}); err == nil || !strings.Contains(err.Error(), "read model file") {
		t.Fatalf("expected missing model error, got %v", err)
	}
}

func TestGeneratedModelFromParsedEntityMetadata(t *testing.T) {
	generated := generatedModelFromParsedEntity("Membership", "memberships", "example.com/app", []parsedField{
		{Name: "TenantID", TypeStr: "uuid.UUID", BunTag: "tenant_id,pk"},
		{Name: "OwnerID", TypeStr: "*uuid.UUID", BunTag: "owner_id"},
		{Name: "Note", TypeStr: "sql.NullString", BunTag: "note"},
		{Name: "CreatedAt", TypeStr: "time.Time", BunTag: "created_at"},
		{Name: "UpdatedAt", TypeStr: "time.Time", BunTag: "updated_at"},
	})
	if !generated.HasPrimaryKey || generated.IDGoFieldName != "TenantID" || generated.IDType != "uuid.UUID" {
		t.Fatalf("primary key metadata was not detected: %#v", generated)
	}
	if !generated.HasCreatedAt || !generated.HasUpdatedAt {
		t.Fatalf("timestamp metadata was not detected: %#v", generated)
	}
	if !generated.Fields[1].IsForeignKey || !generated.Fields[1].IsNullable || !generated.Fields[2].IsNullable {
		t.Fatalf("field metadata was not detected: %#v", generated.Fields)
	}

	withoutPK := generatedModelFromParsedEntity("Log", "logs", "example.com/app", []parsedField{{Name: "Message", TypeStr: "string"}})
	if withoutPK.HasPrimaryKey || withoutPK.IDGoFieldName != "ID" || withoutPK.IDType != "uuid.UUID" {
		t.Fatalf("unexpected fallback primary key metadata: %#v", withoutPK)
	}
}

func TestFactoryCustomImportRetentionAndDeclarationClassification(t *testing.T) {
	source := `package factories

import (
	alias "example.com/alias"
	. "example.com/dot"
	_ "example.com/sideeffect"
	"example.com/version.v2"
)

const CustomValue = alias.Value
`
	custom, imports, err := customFactoryDecls(source, factorySyncGeneratedFactory(), expectedFactoryOptionNames(factorySyncGeneratedFactory()))
	if err != nil {
		t.Fatalf("collect custom declarations: %v", err)
	}
	if !strings.Contains(custom, "CustomValue") {
		t.Fatalf("custom declaration was not retained: %q", custom)
	}
	for _, want := range []string{"example.com/alias", "example.com/dot", "example.com/sideeffect", "example.com/version.v2"} {
		if !containsFactorySyncString(imports, want) {
			t.Fatalf("import %q not retained in %#v", want, imports)
		}
	}

	factory := factorySyncGeneratedFactory()
	parsed, err := parser.ParseFile(token.NewFileSet(), "", "package factories\ntype ProductFactory struct{}\ntype Custom struct{}\n", 0)
	if err != nil {
		t.Fatalf("parse declarations: %v", err)
	}
	if !isGeneratedFactoryDecl(parsed.Decls[0], factory) || isGeneratedFactoryDecl(parsed.Decls[1], factory) {
		t.Fatalf("generated declaration classification failed: %#v", parsed.Decls)
	}
	if (FactorySyncResult{}).HasDrift() {
		t.Fatal("empty result should not have drift")
	}
}

func containsFactorySyncString(values []string, target string) bool {
	return slices.Contains(values, target)
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
