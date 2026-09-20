package migrations

import "testing"

func TestMigrationFormatString(t *testing.T) {
	for _, tt := range []struct {
		format MigrationFormat
		want   string
	}{
		{format: Goose, want: "goose"},
		{format: MigrationFormat(99), want: "unknown"},
	} {
		if got := tt.format.String(); got != tt.want {
			t.Fatalf("MigrationFormat(%d).String() = %q, want %q", tt.format, got, tt.want)
		}
	}
}
