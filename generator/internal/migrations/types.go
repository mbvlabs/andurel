package migrations

// Migration represents migration.
type Migration struct {
	FilePath   string
	Sequence   int
	Name       string
	Format     MigrationFormat
	UpSQL      string
	DownSQL    string
	Statements []string
}

// MigrationFormat represents migration format.
type MigrationFormat int

const (
	// Goose is a constant value for goose.
	Goose MigrationFormat = iota
)

// String performs the string operation.
func (f MigrationFormat) String() string {
	switch f {
	case Goose:
		return "goose"
	default:
		return "unknown"
	}
}
