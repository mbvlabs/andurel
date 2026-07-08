package generator

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"github.com/mbvlabs/andurel/generator/internal/types"
	"github.com/mbvlabs/andurel/generator/internal/validation"
)

// PrimaryKeyInfo represents primary key info.
type PrimaryKeyInfo struct {
	ColumnName      string // SQL column name (e.g., "id", "user_id")
	GoFieldName     string // Go struct field name (e.g., "ID", "UserID")
	DataType        string // SQL data type (e.g., "uuid", "bigint")
	GoType          string // Go type (e.g., "uuid.UUID", "int64")
	IsAutoIncrement bool
	Found           bool
	IsNamedID       bool // Whether the PK column is named "id"
}

// PrimaryKeyResolver represents primary key resolver.
type PrimaryKeyResolver interface {
	ResolveAlternatePK(info PrimaryKeyInfo, tableName string) (PrimaryKeyInfo, error)
	ConfirmNoPK(tableName string) (bool, error)
}

// DefaultPrimaryKeyResolver represents default primary key resolver.
type DefaultPrimaryKeyResolver struct{}

// ResolveAlternatePK resolves alternate primary key.
func (DefaultPrimaryKeyResolver) ResolveAlternatePK(info PrimaryKeyInfo, tableName string) (PrimaryKeyInfo, error) {
	fmt.Printf("\nDetected primary key for table %q:\n", tableName)
	fmt.Printf("  Column: %s (%s)\n", info.ColumnName, info.DataType)
	fmt.Printf("  Go type: %s\n", info.GoType)
	fmt.Print("Is this correct? [Y/n]: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return info, fmt.Errorf("failed to read input: %w", err)
	}
	response = strings.TrimSpace(strings.ToLower(response))

	if response == "" || response == "y" || response == "yes" {
		return info, nil
	}

	return info, fmt.Errorf("generation aborted: primary key not confirmed by user")
}

// ConfirmNoPK performs the confirm no primary key operation.
func (DefaultPrimaryKeyResolver) ConfirmNoPK(tableName string) (bool, error) {
	fmt.Printf("\nTable %q has no primary key defined.\n", tableName)
	fmt.Print("Generate model without primary key? (only Create/All/Paginate will be generated) [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read input: %w", err)
	}
	response = strings.TrimSpace(strings.ToLower(response))

	return response == "y" || response == "yes", nil
}

// NopPrimaryKeyResolver represents nop primary key resolver.
type NopPrimaryKeyResolver struct{}

// ResolveAlternatePK resolves alternate primary key.
func (NopPrimaryKeyResolver) ResolveAlternatePK(info PrimaryKeyInfo, _ string) (PrimaryKeyInfo, error) {
	return info, nil
}

// ConfirmNoPK performs the confirm no primary key operation.
func (NopPrimaryKeyResolver) ConfirmNoPK(_ string) (bool, error) {
	return true, nil
}

// DetectPrimaryKey detects primary key.
func DetectPrimaryKey(cat *catalog.Catalog, tableName string) PrimaryKeyInfo {
	table, err := cat.GetTable("", tableName)
	if err != nil {
		return PrimaryKeyInfo{Found: false}
	}

	var foundIDPK *catalog.Column
	var foundAnyPK *catalog.Column

	for _, col := range table.Columns {
		if col.IsPrimaryKey {
			if col.Name == "id" {
				foundIDPK = col
				break
			}
			if foundAnyPK == nil {
				foundAnyPK = col
			}
		}
	}

	pkCol := foundIDPK
	if pkCol == nil {
		pkCol = foundAnyPK
	}

	if pkCol == nil {
		return PrimaryKeyInfo{Found: false}
	}

	pkType, _ := validation.ClassifyPrimaryKeyType(pkCol.DataType)

	return PrimaryKeyInfo{
		ColumnName:      pkCol.Name,
		GoFieldName:     types.FormatFieldName(pkCol.Name),
		DataType:        pkCol.DataType,
		GoType:          validation.GoType(pkType),
		IsAutoIncrement: validation.IsAutoIncrement(pkCol.DataType),
		Found:           true,
		IsNamedID:       pkCol.Name == "id",
	}
}
