package models

import (
	"fmt"
	"strings"
)

// PlanQuerySource renders the narsilc SQL file for a generated model.
func (g *Generator) PlanQuerySource(model *GeneratedModel) (string, error) {
	if model == nil {
		return "", fmt.Errorf("model is required")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "-- narsilc queries for %s\n", model.EntityName)

	if model.HasPrimaryKey && model.Mode != ModelModeCreateOnly {
		fmt.Fprintf(&b, "\n-- name: Get%s :one\n", model.EntityName)
		fmt.Fprintf(&b, "SELECT *\nFROM %s\nWHERE %s = $1\nLIMIT 1;\n", model.TableName, model.IDFieldName)
	}

	if model.Mode != ModelModeCreateOnly {
		fmt.Fprintf(&b, "\n-- name: List%s :many\n", model.PluralName)
		fmt.Fprintf(&b, "-- @order %s\n", listOrderColumn(model))
		fmt.Fprintf(&b, "SELECT *\nFROM %s;\n", model.TableName)

		fmt.Fprintf(&b, "\n-- name: Count%s :one\n", model.PluralName)
		fmt.Fprintf(&b, "SELECT count(*)\nFROM %s;\n", model.TableName)
	}

	if model.Mode != ModelModeReadOnly {
		insertFields := insertFields(model)
		fmt.Fprintf(&b, "\n-- name: Create%s :one\n", model.EntityName)
		fmt.Fprintf(&b, "INSERT INTO %s (\n", model.TableName)
		writeColumnList(&b, insertFields)
		b.WriteString(") VALUES (\n")
		writePlaceholders(&b, len(insertFields))
		b.WriteString(")\nRETURNING *;\n")
	}

	if model.HasPrimaryKey && model.Mode == ModelModeCRUD {
		updateFields := updateFields(model)
		fmt.Fprintf(&b, "\n-- name: Update%s :one\n", model.EntityName)
		fmt.Fprintf(&b, "UPDATE %s\nSET\n", model.TableName)
		for i, field := range updateFields {
			fmt.Fprintf(&b, "	%s = $%d", field.ColumnName, i+2)
			if i < len(updateFields)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "WHERE %s = $1\nRETURNING *;\n", model.IDFieldName)

		fmt.Fprintf(&b, "\n-- name: Delete%s :exec\n", model.EntityName)
		fmt.Fprintf(&b, "DELETE FROM %s\nWHERE %s = $1;\n", model.TableName, model.IDFieldName)

		fmt.Fprintf(&b, "\n-- name: Upsert%s :one\n", model.EntityName)
		fmt.Fprintf(&b, "INSERT INTO %s (\n", model.TableName)
		writeColumnList(&b, model.Fields)
		b.WriteString(") VALUES (\n")
		writePlaceholders(&b, len(model.Fields))
		fmt.Fprintf(&b, ")\nON CONFLICT (%s) DO UPDATE SET\n", model.IDFieldName)
		conflictFields := upsertConflictFields(model)
		for i, field := range conflictFields {
			fmt.Fprintf(&b, "	%s = excluded.%s", field.ColumnName, field.ColumnName)
			if i < len(conflictFields)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString("RETURNING *;\n")
	}

	return b.String(), nil
}

// listOrderColumn picks a stable column for narsilc query-builder @order.
// Builders require @filter or @order; Limit/Offset alone is not enough.
func listOrderColumn(model *GeneratedModel) string {
	if model.HasPrimaryKey && model.IDFieldName != "" {
		return model.IDFieldName
	}
	for _, field := range model.Fields {
		if field.ColumnName == "created_at" {
			return field.ColumnName
		}
	}
	if len(model.Fields) > 0 {
		return model.Fields[0].ColumnName
	}
	return "id"
}

func insertFields(model *GeneratedModel) []GeneratedField {
	fields := make([]GeneratedField, 0, len(model.Fields))
	for _, field := range model.Fields {
		if field.IsPrimaryKey && model.IsAutoIncrementID {
			continue
		}
		fields = append(fields, field)
	}
	return fields
}

func updateFields(model *GeneratedModel) []GeneratedField {
	fields := make([]GeneratedField, 0, len(model.Fields))
	for _, field := range model.Fields {
		if field.IsPrimaryKey || field.Name == "CreatedAt" {
			continue
		}
		fields = append(fields, field)
	}
	return fields
}

func upsertConflictFields(model *GeneratedModel) []GeneratedField {
	fields := make([]GeneratedField, 0, len(model.Fields))
	for _, field := range model.Fields {
		if field.IsPrimaryKey || field.Name == "CreatedAt" {
			continue
		}
		fields = append(fields, field)
	}
	return fields
}

func writeColumnList(b *strings.Builder, fields []GeneratedField) {
	for i, field := range fields {
		b.WriteString("	")
		b.WriteString(field.ColumnName)
		if i < len(fields)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
}

func writePlaceholders(b *strings.Builder, count int) {
	for i := 1; i <= count; i++ {
		fmt.Fprintf(b, "	$%d", i)
		if i < count {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
}
