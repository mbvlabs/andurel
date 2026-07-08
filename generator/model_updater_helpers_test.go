package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/generator/models"
)

func TestUpdateModelResultDiffsAndStructHelpers(t *testing.T) {
	result := &UpdateModelResult{
		OldStruct:         "type ProductEntity struct {\n\tbun.BaseModel `bun:\"table:products,alias:products\"`\n\n\tName string `bun:\"name\"`\n}\n",
		NewStruct:         "type ProductEntity struct {\n\tbun.BaseModel `bun:\"table:products,alias:products\"`\n\n\tName string `bun:\"name\"`\n\tSku string `bun:\"sku\"`\n}\n",
		OldFactoryContent: "package factories\n\nfunc Old() {}\n",
		NewFactoryContent: "package factories\n\nfunc New() {}\n",
	}

	diff, err := result.Diff()
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if strings.Contains(diff, "bun.BaseModel") || !strings.Contains(diff, "+\tSku string") {
		t.Fatalf("unexpected model diff:\n%s", diff)
	}

	factoryDiff, err := result.FactoryDiff()
	if err != nil {
		t.Fatalf("FactoryDiff: %v", err)
	}
	if !strings.Contains(factoryDiff, "-func Old") || !strings.Contains(factoryDiff, "+func New") {
		t.Fatalf("unexpected factory diff:\n%s", factoryDiff)
	}

	dropped := dropBaseModelLine("type T struct {\n\tbun.BaseModel `bun:\"table:t\"`\n\n\tName string\n}")
	if strings.Contains(dropped, "BaseModel") || strings.Contains(dropped, "\n\n\tName") {
		t.Fatalf("dropBaseModelLine did not remove embedding and blank line:\n%s", dropped)
	}

	rendered := renderEntityStruct("ProductEntity", "products", []models.GeneratedField{
		{Name: "ID", Type: "uuid.UUID", BunTag: "id,pk,type:uuid"},
		{Name: "Name", Type: "ProductName", BunTag: "name"},
	})
	for _, want := range []string{
		"type ProductEntity struct",
		"bun.BaseModel `bun:\"table:products,alias:products\"`",
		"ID uuid.UUID `bun:\"id,pk,type:uuid\"`",
		"Name ProductName `bun:\"name\"`",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered struct missing %q:\n%s", want, rendered)
		}
	}
}

func TestParseEntityStruct(t *testing.T) {
	src := []byte(`package models

import "github.com/uptrace/bun"

type ProductEntity struct {
	bun.BaseModel ` + "`bun:\"table:products,alias:products\"`" + `

	ID uuid.UUID ` + "`bun:\"id,pk,type:uuid\"`" + `
	Name ProductName ` + "`bun:\"name\"`" + `
	Notes *string ` + "`bun:\"notes\"`" + `
}
`)

	fields, start, end, err := parseEntityStruct(src, "ProductEntity")
	if err != nil {
		t.Fatalf("parseEntityStruct: %v", err)
	}
	if start <= 0 || end <= start {
		t.Fatalf("unexpected offsets start=%d end=%d", start, end)
	}
	if len(fields) != 3 {
		t.Fatalf("fields = %#v", fields)
	}
	if !fields[1].IsCustom || fields[1].TypeStr != "ProductName" || fields[1].BunTag != "name" {
		t.Fatalf("custom field not detected: %#v", fields[1])
	}
	if fields[2].IsCustom {
		t.Fatalf("*string should be standard: %#v", fields[2])
	}

	if _, _, _, err := parseEntityStruct([]byte("package models\nfunc bad("), "ProductEntity"); err == nil ||
		!strings.Contains(err.Error(), "failed to parse") {
		t.Fatalf("expected parse error, got %v", err)
	}
	if _, _, _, err := parseEntityStruct([]byte("package models\ntype Other struct{}\n"), "ProductEntity"); err == nil ||
		!strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected missing entity error, got %v", err)
	}
}

func TestRenderCreateAndUpdateDataStructs(t *testing.T) {
	model := &models.GeneratedModel{
		IDGoFieldName:     "AccountID",
		IDType:            "int64",
		IsAutoIncrementID: false,
		Fields: []models.GeneratedField{
			{Name: "AccountID", Type: "int64"},
			{Name: "Name", Type: "string"},
			{Name: "CreatedAt", Type: "time.Time"},
			{Name: "UpdatedAt", Type: "time.Time"},
		},
	}

	createData := renderCreateDataStruct("Account", model)
	for _, want := range []string{
		"type CreateAccountData struct",
		"Name string",
		"AccountID int64",
	} {
		if !strings.Contains(createData, want) {
			t.Fatalf("CreateData missing %q:\n%s", want, createData)
		}
	}
	for _, notWant := range []string{"CreatedAt", "UpdatedAt"} {
		if strings.Contains(createData, notWant) {
			t.Fatalf("CreateData should omit %q:\n%s", notWant, createData)
		}
	}

	model.IsAutoIncrementID = true
	createData = renderCreateDataStruct("Account", model)
	if strings.Contains(createData, "AccountID int64") {
		t.Fatalf("CreateData should omit auto-increment ID:\n%s", createData)
	}

	model.IDGoFieldName = ""
	model.IDType = ""
	updateData := renderUpdateDataStruct("Account", model)
	for _, want := range []string{
		"type UpdateAccountData struct",
		"ID uuid.UUID",
		"Name string",
		"UpdatedAt time.Time",
	} {
		if !strings.Contains(updateData, want) {
			t.Fatalf("UpdateData missing %q:\n%s", want, updateData)
		}
	}
	if strings.Contains(updateData, "CreatedAt") {
		t.Fatalf("UpdateData should omit CreatedAt:\n%s", updateData)
	}
}

func TestFindFuncAndStructOffsets(t *testing.T) {
	src := []byte(`package models

type ProductEntity struct {
	ID string
}

type Alias string

func (p ProductEntity) Create() error {
	return nil
}

func (p *ProductEntity) Update() error {
	return nil
}

func Create() error {
	return nil
}
`)

	start, end, err := findFuncOffsets(src, "ProductEntity", "Create")
	if err != nil {
		t.Fatalf("findFuncOffsets Create: %v", err)
	}
	if got := string(src[start:end]); !strings.Contains(got, "func (p ProductEntity) Create()") {
		t.Fatalf("unexpected Create offsets:\n%s", got)
	}

	start, end, err = findFuncOffsets(src, "ProductEntity", "Update")
	if err != nil {
		t.Fatalf("findFuncOffsets Update: %v", err)
	}
	if got := string(src[start:end]); !strings.Contains(got, "func (p *ProductEntity) Update()") {
		t.Fatalf("unexpected Update offsets:\n%s", got)
	}

	if _, _, err := findFuncOffsets(src, "ProductEntity", "Delete"); err == nil ||
		!strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected missing func error, got %v", err)
	}
	if _, _, err := findFuncOffsets([]byte("package models\nfunc bad("), "ProductEntity", "Create"); err == nil ||
		!strings.Contains(err.Error(), "failed to parse") {
		t.Fatalf("expected func parse error, got %v", err)
	}

	start, end, err = findStructOffsets(src, "ProductEntity")
	if err != nil {
		t.Fatalf("findStructOffsets: %v", err)
	}
	if got := string(src[start:end]); !strings.Contains(got, "type ProductEntity struct") {
		t.Fatalf("unexpected struct offsets:\n%s", got)
	}
	if _, _, err := findStructOffsets(src, "Alias"); err == nil ||
		!strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected non-struct type to be ignored, got %v", err)
	}
	if _, _, err := findStructOffsets([]byte("package models\nfunc bad("), "ProductEntity"); err == nil ||
		!strings.Contains(err.Error(), "failed to parse") {
		t.Fatalf("expected struct parse error, got %v", err)
	}
}

func TestApplyModelUpdateWritesModelAndFactory(t *testing.T) {
	manager, cleanup := setupModelManagerTest(t)
	defer cleanup()

	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	modelPath := filepath.Join(root, "models", "product.go")
	factoryPath := filepath.Join(root, "models", "factories", "product.go")

	result := &UpdateModelResult{
		ModelPath: modelPath,
		NewFileContent: `package models

type ProductEntity struct {
	Name string
}
`,
		FactoryPath: factoryPath,
		NewFactoryContent: `package factories

func Product() {}
`,
	}
	if err := manager.ApplyModelUpdate(result); err != nil {
		t.Fatalf("ApplyModelUpdate: %v", err)
	}
	if data, err := os.ReadFile(modelPath); err != nil || !strings.Contains(string(data), "type ProductEntity struct") {
		t.Fatalf("model write data=%q err=%v", string(data), err)
	}
	if data, err := os.ReadFile(factoryPath); err != nil || !strings.Contains(string(data), "func Product()") {
		t.Fatalf("factory write data=%q err=%v", string(data), err)
	}
}

func TestApplyModelUpdateErrorPaths(t *testing.T) {
	manager, cleanup := setupModelManagerTest(t)
	defer cleanup()

	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	err = manager.ApplyModelUpdate(&UpdateModelResult{
		ModelPath:      filepath.Join(root, "missing", "product.go"),
		NewFileContent: "package models\n",
	})
	if err == nil || !strings.Contains(err.Error(), "failed to write model file") {
		t.Fatalf("expected model write error, got %v", err)
	}

	modelPath := filepath.Join(root, "models", "bad.go")
	err = manager.ApplyModelUpdate(&UpdateModelResult{
		ModelPath:      modelPath,
		NewFileContent: "package models\n",
		FactoryPath:    filepath.Join(root, "models", "factories"),
		NewFactoryContent: `package factories

func Broken(
`,
	})
	if err == nil || !strings.Contains(err.Error(), "failed to format factory file") {
		t.Fatalf("expected factory format error, got %v", err)
	}
}
