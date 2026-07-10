package upgrade

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/mbvlabs/andurel/layout"
	"github.com/pmezard/go-difflib/difflib"
)

const targetLockSchemaVersion = 1

// FileDiff is a deterministic unified diff for one planned path.
type FileDiff struct {
	Path string `json:"path"`
	Diff string `json:"diff"`
}

type plannedFile struct {
	path    string
	before  []byte
	after   []byte
	mode    os.FileMode
	remove  bool
	isLock  bool
	created bool
}

type migrationPlan struct {
	fromVersion    string
	toVersion      string
	sourceSchema   int
	targetSchema   int
	dirty          bool
	files          []plannedFile
	lockMigrations []string
	toolChanges    ToolSyncResult
	conflicts      []string
	diffs          []FileDiff
}

func (p *migrationPlan) cloneReport() *UpgradeReport {
	report := &UpgradeReport{
		FromVersion:         p.fromVersion,
		ToVersion:           p.toVersion,
		DirtyWorktree:       p.dirty,
		LockMigrations:      slices.Clone(p.lockMigrations),
		AddedTools:          slices.Clone(p.toolChanges.Added),
		RemovedTools:        slices.Clone(p.toolChanges.Removed),
		UpdatedTools:        slices.Clone(p.toolChanges.Updated),
		ToolMetadataChanges: slices.Clone(p.toolChanges.Metadata),
		Conflicts:           slices.Clone(p.conflicts),
		Diffs:               slices.Clone(p.diffs),
	}
	report.ToolsAdded = len(report.AddedTools)
	report.ToolsRemoved = len(report.RemovedTools)
	report.ToolsUpdated = len(report.UpdatedTools)
	for _, file := range p.files {
		if file.isLock {
			continue
		}
		if file.remove {
			report.RemovedFiles = append(report.RemovedFiles, file.path)
		} else {
			report.ReplacedFiles = append(report.ReplacedFiles, file.path)
		}
	}
	report.FilesReplaced = len(report.ReplacedFiles)
	report.FilesRemoved = len(report.RemovedFiles)
	return report
}

func (u *Upgrader) buildPlan(dirty bool) (*migrationPlan, error) {
	lock, err := cloneLock(u.lock)
	if err != nil {
		return nil, fmt.Errorf("clone lock: %w", err)
	}
	plan := &migrationPlan{
		fromVersion:  u.lock.Version,
		toVersion:    u.opts.TargetVersion,
		sourceSchema: u.sourceLockSchema,
		targetSchema: targetLockSchemaVersion,
		dirty:        dirty,
	}

	selected := selectMigrations(MigrationSelector{
		SourceFrameworkVersion: u.lock.Version,
		TargetFrameworkVersion: u.opts.TargetVersion,
		SourceLockSchema:       u.sourceLockSchema,
		TargetLockSchema:       targetLockSchemaVersion,
	})
	for _, migration := range selected {
		if migration.Kind == MigrationKindLockSchema {
			plan.lockMigrations = append(plan.lockMigrations, migration.Name)
		}
	}

	toolChanges, err := syncTools(lock)
	if err != nil {
		return nil, fmt.Errorf("plan tool metadata: %w", err)
	}
	plan.toolChanges = *toolChanges
	if lock.DatabaseConfig == nil {
		lock.DatabaseConfig = &layout.DatabaseConfig{NullType: "sql.Null"}
		plan.lockMigrations = append(plan.lockMigrations, "add-database-config")
	}
	lock.SchemaVersion = targetLockSchemaVersion

	rendered, err := u.generator.RenderFrameworkTemplates(
		u.projectRoot,
		*u.lock.ScaffoldConfig,
		u.lock.ExtensionNames(),
	)
	if err != nil {
		return nil, fmt.Errorf("render framework templates: %w", err)
	}
	paths := make([]string, 0, len(rendered))
	for path := range rendered {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if recognized, recognitionErr := recognizeWholeFileReplacement(u.projectRoot, path, rendered[path]); recognitionErr != nil {
			return nil, recognitionErr
		} else if !recognized {
			continue
		}
		if err := plan.addReplacement(u.projectRoot, path, rendered[path], false); err != nil {
			return nil, err
		}
	}

	obsolete := u.obsoleteManagedInternalFiles()
	sort.Strings(obsolete)
	for _, path := range obsolete {
		if recognized, recognitionErr := recognizeWholeFileDeletion(u.projectRoot, path); recognitionErr != nil {
			return nil, recognitionErr
		} else if !recognized {
			continue
		}
		if err := plan.addDeletion(u.projectRoot, path); err != nil {
			return nil, err
		}
	}

	lock.Version = u.opts.TargetVersion
	lockBytes, err := marshalLock(lock)
	if err != nil {
		return nil, fmt.Errorf("render final lock: %w", err)
	}
	if err := plan.addReplacement(u.projectRoot, "andurel.lock", lockBytes, true); err != nil {
		return nil, err
	}

	sort.Strings(plan.conflicts)
	sort.SliceStable(plan.files, func(i, j int) bool {
		if plan.files[i].isLock != plan.files[j].isLock {
			return !plan.files[i].isLock
		}
		return plan.files[i].path < plan.files[j].path
	})
	plan.diffs = make([]FileDiff, 0, len(plan.files))
	for _, file := range plan.files {
		diff, diffErr := unifiedFileDiff(file)
		if diffErr != nil {
			return nil, diffErr
		}
		if diff != "" {
			plan.diffs = append(plan.diffs, FileDiff{Path: file.path, Diff: diff})
		}
	}
	return plan, nil
}

func recognizeWholeFileReplacement(root, path string, target []byte) (bool, error) {
	current, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if os.IsNotExist(err) {
		return hasAndurelVersionMarker(target), nil
	}
	if err != nil {
		return false, err
	}
	if bytes.Equal(current, target) {
		return true, nil
	}
	return hasAndurelVersionMarker(current), nil
}

func recognizeWholeFileDeletion(root, path string) (bool, error) {
	current, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return hasAndurelVersionMarker(current), nil
}

func hasAndurelVersionMarker(content []byte) bool {
	const prefix = "// Code generated by andurel "
	const suffix = "; DO NOT EDIT."

	for line := range bytes.Lines(content) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte(prefix)) || !bytes.HasSuffix(line, []byte(suffix)) {
			continue
		}
		version := line[len(prefix) : len(line)-len(suffix)]
		return len(bytes.TrimSpace(version)) > 0
	}
	return false
}

func (p *migrationPlan) addReplacement(root, path string, after []byte, isLock bool) error {
	for index := range p.files {
		if p.files[index].path != path {
			continue
		}
		if !bytes.Equal(p.files[index].after, after) || p.files[index].remove {
			p.conflicts = append(p.conflicts, fmt.Sprintf("%s has competing planned transformations", path))
		}
		return nil
	}
	fullPath := filepath.Join(root, path)
	before, err := os.ReadFile(fullPath)
	created := false
	mode := os.FileMode(0o644)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("read %s: %w", path, err)
		}
		created = true
		before = nil
	} else if info, statErr := os.Stat(fullPath); statErr != nil {
		return fmt.Errorf("stat %s: %w", path, statErr)
	} else {
		mode = info.Mode().Perm()
	}
	if bytes.Equal(before, after) {
		return nil
	}
	p.files = append(p.files, plannedFile{
		path: path, before: slices.Clone(before), after: slices.Clone(after),
		mode: mode, isLock: isLock, created: created,
	})
	return nil
}

func (p *migrationPlan) addDeletion(root, path string) error {
	fullPath := filepath.Join(root, path)
	before, err := os.ReadFile(fullPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read deletion %s: %w", path, err)
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return fmt.Errorf("stat deletion %s: %w", path, err)
	}
	p.files = append(p.files, plannedFile{path: path, before: before, mode: info.Mode().Perm(), remove: true})
	return nil
}

func cloneLock(lock *layout.AndurelLock) (*layout.AndurelLock, error) {
	data, err := json.Marshal(lock)
	if err != nil {
		return nil, err
	}
	var result layout.AndurelLock
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func marshalLock(lock *layout.AndurelLock) ([]byte, error) {
	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func unifiedFileDiff(file plannedFile) (string, error) {
	before := strings.SplitAfter(string(file.before), "\n")
	after := strings.SplitAfter(string(file.after), "\n")
	if file.remove {
		after = nil
	}
	return difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A: before, B: after, FromFile: "a/" + file.path, ToFile: "b/" + file.path, Context: 3,
	})
}
