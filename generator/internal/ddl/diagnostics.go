package ddl

import (
	"fmt"
	"strings"
)

// UnsupportedStatementError identifies DDL that may change generated models
// but cannot be represented safely by the catalog parser.
type UnsupportedStatementError struct {
	Statement string
	Reason    string
}

// Error returns an actionable parser diagnostic.
func (e *UnsupportedStatementError) Error() string {
	return fmt.Sprintf(
		"unsupported schema-changing DDL: %s; %s; split the migration into supported CREATE TABLE, ALTER TABLE, or DROP TABLE statements",
		statementPreview(e.Statement),
		e.Reason,
	)
}

func unsupportedStatement(sql, reason string) error {
	return &UnsupportedStatementError{Statement: strings.TrimSpace(sql), Reason: reason}
}

func statementPreview(sql string) string {
	preview := strings.Join(strings.Fields(sql), " ")
	const maxLength = 120
	if len(preview) > maxLength {
		return preview[:maxLength] + "..."
	}
	return preview
}

func unknownStatementIsModelNeutral(sql string) bool {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(sql)))
	if len(fields) == 0 {
		return true
	}
	switch fields[0] {
	case "analyze", "begin", "comment", "commit", "delete", "grant", "insert", "release", "reset", "revoke", "rollback", "savepoint", "set", "truncate", "update", "vacuum":
		return true
	case "select":
		return !strings.Contains(strings.ToLower(sql), "into")
	default:
		return false
	}
}

func validateDDLStructure(sql string) error {
	parenDepth := 0
	inSingleQuote := false
	inDoubleQuote := false
	inLineComment := false
	inBlockComment := false
	statementEnded := false

	for i := 0; i < len(sql); i++ {
		char := sql[i]
		next := byte(0)
		if i+1 < len(sql) {
			next = sql[i+1]
		}

		if inLineComment {
			if char == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if char == '*' && next == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if inSingleQuote {
			if char == '\'' {
				if next == '\'' {
					i++
				} else {
					inSingleQuote = false
				}
			}
			continue
		}
		if inDoubleQuote {
			if char == '"' {
				if next == '"' {
					i++
				} else {
					inDoubleQuote = false
				}
			}
			continue
		}

		if statementEnded {
			if char == '-' && next == '-' {
				inLineComment = true
				i++
				continue
			}
			if char == '/' && next == '*' {
				inBlockComment = true
				i++
				continue
			}
			if !isSQLSpace(char) {
				return unsupportedStatement(sql, "multiple top-level SQL statements are structurally ambiguous")
			}
			continue
		}
		switch {
		case char == '-' && next == '-':
			inLineComment = true
			i++
		case char == '/' && next == '*':
			inBlockComment = true
			i++
		case char == '\'':
			inSingleQuote = true
		case char == '"':
			inDoubleQuote = true
		case char == '(':
			parenDepth++
		case char == ')':
			parenDepth--
			if parenDepth < 0 {
				return unsupportedStatement(sql, "closing parenthesis has no matching opening parenthesis")
			}
		case char == ';' && parenDepth == 0:
			statementEnded = true
		}
	}

	switch {
	case inSingleQuote:
		return unsupportedStatement(sql, "single-quoted string is not terminated")
	case inDoubleQuote:
		return unsupportedStatement(sql, "double-quoted identifier is not terminated")
	case inBlockComment:
		return unsupportedStatement(sql, "block comment is not terminated")
	case parenDepth != 0:
		return unsupportedStatement(sql, "parentheses are unbalanced")
	default:
		return nil
	}
}

func isSQLSpace(char byte) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '\r'
}
