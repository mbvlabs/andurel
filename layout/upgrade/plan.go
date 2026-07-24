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

type upgradePlan struct {
	fromVersion   string
	toVersion     string
	dirty         bool
	files         []plannedFile
	toolChanges   ToolSyncResult
	diffs         []FileDiff
	manualActions []ManualAction
}

func (p *upgradePlan) cloneReport() *UpgradeReport {
	report := &UpgradeReport{
		FromVersion:         p.fromVersion,
		ToVersion:           p.toVersion,
		DirtyWorktree:       p.dirty,
		AddedTools:          slices.Clone(p.toolChanges.Added),
		RemovedTools:        slices.Clone(p.toolChanges.Removed),
		UpdatedTools:        slices.Clone(p.toolChanges.Updated),
		ToolMetadataChanges: slices.Clone(p.toolChanges.Metadata),
		Diffs:               slices.Clone(p.diffs),
		ManualActions:       slices.Clone(p.manualActions),
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

func (u *Upgrader) buildPlan(dirty bool) (*upgradePlan, error) {
	lock, err := cloneLock(u.lock)
	if err != nil {
		return nil, fmt.Errorf("clone lock: %w", err)
	}
	plan := &upgradePlan{
		fromVersion: u.lock.Version,
		toVersion:   u.opts.TargetVersion,
		dirty:       dirty,
	}
	if crossesVersion(plan.fromVersion, plan.toVersion, sessionCookieRecoveryVersion) {
		modulePath, err := resolveModulePath(u.projectRoot)
		if err != nil {
			return nil, fmt.Errorf("resolve module path for manual actions: %w", err)
		}
		plan.manualActions, err = manualActionsForUpgrade(
			plan.fromVersion,
			plan.toVersion,
			modulePath,
		)
		if err != nil {
			return nil, err
		}
	}

	toolChanges, err := syncTools(lock)
	if err != nil {
		return nil, fmt.Errorf("plan tool metadata: %w", err)
	}
	plan.toolChanges = *toolChanges
	if lock.DatabaseConfig == nil {
		lock.DatabaseConfig = &layout.DatabaseConfig{NullType: "sql.Null"}
	}
	lock.SchemaVersion = targetLockSchemaVersion

	if err := u.addInertiaRootMigration(plan, lock); err != nil {
		return nil, err
	}
	if err := u.addFrameworkChanges(plan); err != nil {
		return nil, err
	}

	lock.Version = u.opts.TargetVersion
	lockBytes, err := marshalLock(lock)
	if err != nil {
		return nil, fmt.Errorf("render final lock: %w", err)
	}
	if err := plan.addReplacement(u.projectRoot, "andurel.lock", lockBytes, true); err != nil {
		return nil, err
	}

	if err := finalizePlan(plan); err != nil {
		return nil, err
	}
	return plan, nil
}

func (u *Upgrader) buildRepairPlan(dirty bool) (*upgradePlan, error) {
	plan := &upgradePlan{
		fromVersion: u.lock.Version,
		toVersion:   u.opts.TargetVersion,
		dirty:       dirty,
	}
	if err := u.addFrameworkChanges(plan); err != nil {
		return nil, err
	}
	if err := finalizePlan(plan); err != nil {
		return nil, err
	}
	return plan, nil
}

func (u *Upgrader) addInertiaRootMigration(plan *upgradePlan, lock *layout.AndurelLock) error {
	if !layout.IsSupportedInertiaAdapter(lock.ScaffoldConfig.Inertia) {
		return nil
	}

	const embeddedPath = "assets/inertia/root.go.html"
	if _, err := os.Stat(filepath.Join(u.projectRoot, embeddedPath)); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect embedded Inertia root: %w", err)
	}

	const legacyPath = "views/root.go.html"
	rootHTML, err := os.ReadFile(filepath.Join(u.projectRoot, legacyPath))
	if err != nil {
		return fmt.Errorf("read existing Inertia root %s: %w", legacyPath, err)
	}
	if err := plan.addReplacement(u.projectRoot, embeddedPath, rootHTML, false); err != nil {
		return fmt.Errorf("embed existing Inertia root: %w", err)
	}
	if err := plan.addDeletion(u.projectRoot, legacyPath); err != nil {
		return fmt.Errorf("remove existing Inertia root: %w", err)
	}

	const mainPath = "cmd/app/main.go"
	mainFile, err := os.ReadFile(filepath.Join(u.projectRoot, mainPath))
	if err != nil {
		return fmt.Errorf("read Inertia application entrypoint: %w", err)
	}
	updatedMain := bytes.Replace(mainFile,
		[]byte(`inertia.Init("views/root.go.html")`),
		[]byte(`inertia.Init("inertia/root.go.html")`),
		1,
	)
	if bytes.Equal(mainFile, updatedMain) {
		return fmt.Errorf("update Inertia application entrypoint: legacy initialization call not found")
	}
	if err := plan.addReplacement(u.projectRoot, mainPath, updatedMain, false); err != nil {
		return fmt.Errorf("update Inertia application entrypoint: %w", err)
	}
	return nil
}

func (u *Upgrader) addFrameworkChanges(plan *upgradePlan) error {
	rendered, err := u.generator.RenderFrameworkTemplates(
		u.projectRoot,
		*u.lock.ScaffoldConfig,
		u.lock.ExtensionNames(),
	)
	if err != nil {
		return fmt.Errorf("render framework templates: %w", err)
	}
	paths := make([]string, 0, len(rendered))
	for path := range rendered {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if recognized, recognitionErr := recognizeWholeFileReplacement(u.projectRoot, path, rendered[path]); recognitionErr != nil {
			return recognitionErr
		} else if !recognized {
			continue
		}
		if err := plan.addReplacement(u.projectRoot, path, rendered[path], false); err != nil {
			return err
		}
	}

	obsolete := u.obsoleteManagedInternalFiles()
	sort.Strings(obsolete)
	for _, path := range obsolete {
		if recognized, recognitionErr := recognizeWholeFileDeletion(u.projectRoot, path); recognitionErr != nil {
			return recognitionErr
		} else if !recognized {
			continue
		}
		if err := plan.addDeletion(u.projectRoot, path); err != nil {
			return err
		}
	}
	return nil
}

func finalizePlan(plan *upgradePlan) error {
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
			return diffErr
		}
		if diff != "" {
			plan.diffs = append(plan.diffs, FileDiff{Path: file.path, Diff: diff})
		}
	}
	return nil
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

func (p *upgradePlan) addReplacement(root, path string, after []byte, isLock bool) error {
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

func (p *upgradePlan) addDeletion(root, path string) error {
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
