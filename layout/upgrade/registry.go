package upgrade

import (
	"sort"
	"strings"

	"github.com/mbvlabs/andurel/layout"
)

// MigrationKind identifies the independently selected migration dimension.
type MigrationKind string

const (
	MigrationKindFramework  MigrationKind = "framework"
	MigrationKindLockSchema MigrationKind = "lock-schema"
)

// MigrationSelector keeps framework and lock-schema selection independent.
type MigrationSelector struct {
	SourceFrameworkVersion string
	TargetFrameworkVersion string
	SourceLockSchema       int
	TargetLockSchema       int
}

type transformedFile struct {
	Path    string
	Content []byte
}

// Migration describes one ordered migration registry entry.
type Migration struct {
	Order            int
	Name             string
	Kind             MigrationKind
	SourceFramework  string
	TargetFramework  string
	SourceLockSchema int
	TargetLockSchema int
	Transform        func(string, *TemplateGenerator, *layout.AndurelLock) ([]transformedFile, []string, error)
}

var migrationRegistry = []Migration{
	{Order: 10, Name: "lock-schema-legacy-to-1", Kind: MigrationKindLockSchema, SourceLockSchema: 0, TargetLockSchema: 1},
}

func selectMigrations(selector MigrationSelector) []Migration {
	selected := make([]Migration, 0, len(migrationRegistry))
	for _, migration := range migrationRegistry {
		switch migration.Kind {
		case MigrationKindFramework:
			if migration.SourceFramework == selector.SourceFrameworkVersion &&
				selector.TargetFrameworkVersion != selector.SourceFrameworkVersion &&
				strings.HasPrefix(selector.TargetFrameworkVersion, migration.TargetFramework+".") {
				selected = append(selected, migration)
			}
		case MigrationKindLockSchema:
			if migration.SourceLockSchema == selector.SourceLockSchema &&
				migration.TargetLockSchema == selector.TargetLockSchema {
				selected = append(selected, migration)
			}
		}
	}
	sort.SliceStable(selected, func(i, j int) bool { return selected[i].Order < selected[j].Order })
	return selected
}
