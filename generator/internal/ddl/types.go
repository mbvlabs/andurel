package ddl

import (
	"regexp"
	"strconv"
	"strings"
)

// ParseDataType parses SQL data type strings into components
func ParseDataType(typeStr string) (dataType string, length *int32, precision *int32, scale *int32) {
	typeStrLower := strings.ToLower(typeStr)

	if strings.Contains(typeStrLower, "timestamp with time zone") {
		return "timestamp with time zone", nil, nil, nil
	}
	if strings.Contains(typeStrLower, "timestamp without time zone") {
		return "timestamp without time zone", nil, nil, nil
	}

	// Handle varchar(n)
	varcharRegex, err := regexp.Compile(`varchar\((\d+)\)`)
	if err != nil {
		return strings.TrimSpace(typeStr), nil, nil, nil
	}
	if matches := varcharRegex.FindStringSubmatch(typeStrLower); len(matches) > 1 {
		if n, err := strconv.Atoi(matches[1]); err == nil {
			length := int32(n)
			return "varchar", &length, nil, nil
		}
	}

	// Handle char(n)
	charRegex, err := regexp.Compile(`char\((\d+)\)`)
	if err != nil {
		return strings.TrimSpace(typeStr), nil, nil, nil
	}
	if matches := charRegex.FindStringSubmatch(typeStrLower); len(matches) > 1 {
		if n, err := strconv.Atoi(matches[1]); err == nil {
			length := int32(n)
			return "char", &length, nil, nil
		}
	}

	// Handle decimal(p,s) and numeric(p,s)
	decimalRegex, err := regexp.Compile(`(?:decimal|numeric)\((\d+),(\d+)\)`)
	if err != nil {
		return strings.TrimSpace(typeStr), nil, nil, nil
	}
	if matches := decimalRegex.FindStringSubmatch(typeStrLower); len(matches) > 2 {
		if p, err1 := strconv.Atoi(matches[1]); err1 == nil {
			if s, err2 := strconv.Atoi(matches[2]); err2 == nil {
				precision := int32(p)
				scale := int32(s)
				dataType := "numeric"
				if strings.HasPrefix(typeStrLower, "decimal") {
					dataType = "decimal"
				}
				return dataType, nil, &precision, &scale
			}
		}
	}

	// Simple types without parameters
	switch typeStrLower {
	case "integer", "int", "int4":
		return "integer", nil, nil, nil
	case "bigint", "int8":
		return "bigint", nil, nil, nil
	case "smallint", "int2":
		return "smallint", nil, nil, nil
	case "serial":
		return "serial", nil, nil, nil
	case "bigserial":
		return "bigserial", nil, nil, nil
	case "text":
		return "text", nil, nil, nil
	case "boolean", "bool":
		return "boolean", nil, nil, nil
	case "date":
		return "date", nil, nil, nil
	case "time":
		return "time", nil, nil, nil
	case "timestamp":
		return "timestamp", nil, nil, nil
	case "real", "float4":
		return "real", nil, nil, nil
	case "double precision", "float8":
		return "double precision", nil, nil, nil
	case "uuid":
		return "uuid", nil, nil, nil
	case "json":
		return "json", nil, nil, nil
	case "jsonb":
		return "jsonb", nil, nil, nil
	default:
		return strings.TrimSpace(typeStr), nil, nil, nil
	}
}
