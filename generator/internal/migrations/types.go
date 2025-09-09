package migrations

type Migration struct {
	FilePath   string
	Sequence   int
	Name       string
	Format     MigrationFormat
	UpSQL      string
	DownSQL    string
	Statements []string
}

type MigrationFormat int

const (
	Goose MigrationFormat = iota
)

func (f MigrationFormat) String() string {
	switch f {
	case Goose:
		return "goose"
	default:
		return "unknown"
	}
}
