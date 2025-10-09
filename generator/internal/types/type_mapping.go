package types

import "github.com/mbvlabs/andurel/generator/internal/catalog"

// TypeMapping represents the result of type mapping
type TypeMapping struct {
	GoType      string
	SQLCType    string
	PackageName string
}

// DatabaseTypeMapper interface defines the contract for database-specific type mapping
type DatabaseTypeMapper interface {
	MapToGoType(column *catalog.Column) (TypeMapping, error)
	MapToSQLType(goType string, nullable bool) (string, error)
	GenerateConversionFromDB(fieldName, sqlcType, goType string) string
	GenerateConversionToDB(sqlcType, goType, valueExpr string) string
	GenerateZeroCheck(goType, valueExpr string) string
	GetDatabaseType() string
}

// TypeConversionService manages database-specific type mappers
type TypeConversionService struct {
	mappers map[string]DatabaseTypeMapper
}

// NewTypeConversionService creates a new type conversion service
func NewTypeConversionService() *TypeConversionService {
	service := &TypeConversionService{
		mappers: make(map[string]DatabaseTypeMapper),
	}

	// Register default mappers
	service.RegisterMapper("postgresql", NewPostgreSQLTypeMapper())
	service.RegisterMapper("sqlite", NewSQLiteTypeMapper())

	return service
}

// RegisterMapper registers a new type mapper for a database type
func (tcs *TypeConversionService) RegisterMapper(databaseType string, mapper DatabaseTypeMapper) {
	tcs.mappers[databaseType] = mapper
}

// GetMapper returns the type mapper for the specified database type
func (tcs *TypeConversionService) GetMapper(databaseType string) (DatabaseTypeMapper, error) {
	mapper, exists := tcs.mappers[databaseType]
	if !exists {
		// Default to PostgreSQL mapper for unknown types
		mapper, exists = tcs.mappers["postgresql"]
		if !exists {
			return nil, ErrNoTypeMapperFound
		}
	}
	return mapper, nil
}

// MapToGoType maps a database column to a Go type using the appropriate mapper
func (tcs *TypeConversionService) MapToGoType(databaseType string, column *catalog.Column) (TypeMapping, error) {
	mapper, err := tcs.GetMapper(databaseType)
	if err != nil {
		return TypeMapping{}, err
	}

	return mapper.MapToGoType(column)
}

// MapToSQLType maps a Go type to a SQL type using the appropriate mapper
func (tcs *TypeConversionService) MapToSQLType(databaseType, goType string, nullable bool) (string, error) {
	mapper, err := tcs.GetMapper(databaseType)
	if err != nil {
		return "", err
	}

	return mapper.MapToSQLType(goType, nullable)
}

// GenerateConversionFromDB generates conversion code from database to Go type
func (tcs *TypeConversionService) GenerateConversionFromDB(databaseType, fieldName, sqlcType, goType string) string {
	mapper, err := tcs.GetMapper(databaseType)
	if err != nil {
		return fieldName // Fallback to direct field access
	}

	return mapper.GenerateConversionFromDB(fieldName, sqlcType, goType)
}

// GenerateConversionToDB generates conversion code from Go to database type
func (tcs *TypeConversionService) GenerateConversionToDB(databaseType, sqlcType, goType, valueExpr string) string {
	mapper, err := tcs.GetMapper(databaseType)
	if err != nil {
		return valueExpr // Fallback to direct value
	}

	return mapper.GenerateConversionToDB(sqlcType, goType, valueExpr)
}

// GenerateZeroCheck generates zero-value check code for a Go type
func (tcs *TypeConversionService) GenerateZeroCheck(databaseType, goType, valueExpr string) string {
	mapper, err := tcs.GetMapper(databaseType)
	if err != nil {
		return "true" // Fallback to always true
	}

	return mapper.GenerateZeroCheck(goType, valueExpr)
}
