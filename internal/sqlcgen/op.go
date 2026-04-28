package sqlcgen

import "strings"

// Op mirrors the storage runtime Op enum. The string values are the
// identifier names emitted into generated code (e.g. "OpFind"), so this
// package's notion of an Op stays in lock-step with internal/storage.
type Op string

const (
	OpFind   Op = "OpFind"
	OpCreate Op = "OpCreate"
	OpUpdate Op = "OpUpdate"
	OpDelete Op = "OpDelete"
)

// deriveOp classifies a query's operation from its name prefix. Per spec:
//
//	Query*, Count*               -> OpFind
//	Insert*                      -> OpCreate
//	Update*                      -> OpUpdate
//	Delete*                      -> OpDelete
//	Upsert*                      -> OpUpdate (treated as a write that may modify)
//	otherwise                    -> OpFind (read-leaning default)
func deriveOp(queryName string) Op {
	switch {
	case strings.HasPrefix(queryName, "Insert"):
		return OpCreate
	case strings.HasPrefix(queryName, "Update"):
		return OpUpdate
	case strings.HasPrefix(queryName, "Delete"):
		return OpDelete
	case strings.HasPrefix(queryName, "Upsert"):
		return OpUpdate
	default:
		// Query*, Count*, and any unknown prefix all read-leaning by default.
		return OpFind
	}
}
