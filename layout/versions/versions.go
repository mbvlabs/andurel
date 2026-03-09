package versions

const (
	Templ       = "v0.3.833"
	Sqlc        = "v1.30.0"
	Goose       = "v3.26.0"
	Mailpit     = "v1.29.0"
	Usql        = "v0.20.8"
	Dblab       = "v0.34.2"
	TailwindCLI = "v4.1.18"
	Shadowfax   = "v0.1.3"
)

type ArchiveType string

const (
	ArchiveZip    ArchiveType = "zip"
	ArchiveTarGz  ArchiveType = "tar.gz"
	ArchiveTarBz2 ArchiveType = "tar.bz2"
	ArchiveBinary ArchiveType = "binary"
)

type ToolSpec struct {
	Name        string
	Module      string
	URLTemplate string
	Archive     ArchiveType
	Windows     *OSOverride
}

type OSOverride struct {
	URLTemplate string
	Archive     ArchiveType
}

var Tools = map[string]ToolSpec{
	"templ": {
		Name:        "templ",
		Module:      "github.com/a-h/templ/cmd/templ",
		URLTemplate: "https://github.com/a-h/templ/releases/download/{{version}}/templ_{{os_capitalized}}_{{arch_x86_64}}.tar.gz",
		Archive:     ArchiveTarGz,
		Windows: &OSOverride{
			URLTemplate: "https://github.com/a-h/templ/releases/download/{{version}}/templ_Windows_{{arch_x86_64}}.zip",
			Archive:     ArchiveZip,
		},
	},
	"sqlc": {
		Name:        "sqlc",
		Module:      "github.com/sqlc-dev/sqlc/cmd/sqlc",
		URLTemplate: "https://github.com/sqlc-dev/sqlc/releases/download/{{version}}/sqlc_{{version_no_v}}_{{os}}_{{arch}}.tar.gz",
		Archive:     ArchiveTarGz,
		Windows: &OSOverride{
			URLTemplate: "https://github.com/sqlc-dev/sqlc/releases/download/{{version}}/sqlc_{{version_no_v}}_windows_{{arch}}.zip",
			Archive:     ArchiveZip,
		},
	},
	"goose": {
		Name:        "goose",
		Module:      "github.com/pressly/goose/v3/cmd/goose",
		URLTemplate: "https://github.com/pressly/goose/releases/download/{{version}}/goose_{{os}}_{{arch_x86_64}}",
		Archive:     ArchiveBinary,
		Windows: &OSOverride{
			URLTemplate: "https://github.com/pressly/goose/releases/download/{{version}}/goose_windows_{{arch_x86_64}}.exe",
			Archive:     ArchiveBinary,
		},
	},
	"mailpit": {
		Name:        "mailpit",
		Module:      "github.com/axllent/mailpit",
		URLTemplate: "https://github.com/axllent/mailpit/releases/download/{{version}}/mailpit-{{os}}-{{arch}}.tar.gz",
		Archive:     ArchiveTarGz,
		Windows: &OSOverride{
			URLTemplate: "https://github.com/axllent/mailpit/releases/download/{{version}}/mailpit-windows-{{arch}}.zip",
			Archive:     ArchiveZip,
		},
	},
	"usql": {
		Name:        "usql",
		Module:      "github.com/xo/usql",
		URLTemplate: "https://github.com/xo/usql/releases/download/{{version}}/usql-{{version_no_v}}-{{os}}-{{arch}}.tar.bz2",
		Archive:     ArchiveTarBz2,
		Windows: &OSOverride{
			URLTemplate: "https://github.com/xo/usql/releases/download/{{version}}/usql-{{version_no_v}}-windows-{{arch}}.zip",
			Archive:     ArchiveZip,
		},
	},
	"dblab": {
		Name:        "dblab",
		Module:      "github.com/danvergara/dblab",
		URLTemplate: "https://github.com/danvergara/dblab/releases/download/{{version}}/dblab_{{version_no_v}}_{{os}}_{{arch}}.tar.gz",
		Archive:     ArchiveTarGz,
	},
	"shadowfax": {
		Name:        "shadowfax",
		Module:      "github.com/mbvlabs/shadowfax",
		URLTemplate: "https://github.com/mbvlabs/shadowfax/releases/download/{{version}}/shadowfax-{{os}}-{{arch}}",
		Archive:     ArchiveBinary,
		Windows: &OSOverride{
			URLTemplate: "https://github.com/mbvlabs/shadowfax/releases/download/{{version}}/shadowfax-windows-{{arch}}.zip",
			Archive:     ArchiveZip,
		},
	},
	"tailwindcli": {
		Name:        "tailwindcli",
		URLTemplate: "https://github.com/tailwindlabs/tailwindcss/releases/download/{{version}}/tailwindcss-{{os_tailwind}}-{{arch_tailwind}}",
		Archive:     ArchiveBinary,
		Windows: &OSOverride{
			URLTemplate: "https://github.com/tailwindlabs/tailwindcss/releases/download/{{version}}/tailwindcss-windows-{{arch_tailwind}}.exe",
			Archive:     ArchiveBinary,
		},
	},
}
