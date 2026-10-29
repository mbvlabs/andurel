package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
)

// buildOutput describes where build artifacts are published.
type buildOutput struct {
	AbsPath     string
	DisplayPath string
	Isolated    bool
}

// sourcePathDenyList lists project subtrees that hold build inputs or source
// code. An output directory inside one of them would mix published artifacts
// with inputs that an isolated build must leave untouched.
var sourcePathDenyList = map[string]bool{
	".git":       true,
	"assets":     true,
	"bin":        true,
	"cmd":        true,
	"config":     true,
	"domains":    true,
	"email":      true,
	"handlers":   true,
	"internal":   true,
	"middleware": true,
	"migrations": true,
	"queue":      true,
	"resources":  true,
	"routes":     true,
	"services":   true,
}

// resolveBuildOutputDir validates --output-dir and decides whether the build
// must stage the project. An empty value keeps the legacy in-place layout.
func resolveBuildOutputDir(rootDir, raw string) (buildOutput, error) {
	rootAbs, err := filepath.Abs(rootDir)
	if err != nil {
		return buildOutput{}, fmt.Errorf("resolve project root: %w", err)
	}
	rootAbs, err = resolveBuildPath(rootAbs)
	if err != nil {
		return buildOutput{}, fmt.Errorf("resolve project root: %w", err)
	}

	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return buildOutput{
			AbsPath:     filepath.Join(rootAbs, "dist"),
			DisplayPath: "dist",
			Isolated:    false,
		}, nil
	}

	outputPath := trimmed
	if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(rootAbs, outputPath)
	}
	outputPath = filepath.Clean(outputPath)
	outputPath, err = resolveBuildPath(outputPath)
	if err != nil {
		return buildOutput{}, fmt.Errorf("resolve --output-dir: %w", err)
	}

	if outputPath == rootAbs {
		return buildOutput{}, fmt.Errorf(
			"--output-dir cannot be the project root; choose a dedicated directory",
		)
	}
	if isPathWithin(outputPath, rootAbs) {
		return buildOutput{}, fmt.Errorf(
			"--output-dir cannot contain the project root: %s",
			outputPath,
		)
	}
	if rel, ok := relativeWithin(rootAbs, outputPath); ok {
		first, _, _ := strings.Cut(filepath.ToSlash(rel), "/")
		if sourcePathDenyList[first] {
			return buildOutput{}, fmt.Errorf(
				"--output-dir cannot be inside %s/, which holds build inputs: %s",
				first,
				outputPath,
			)
		}
	}
	if info, statErr := os.Stat(outputPath); statErr == nil && !info.IsDir() {
		return buildOutput{}, fmt.Errorf("--output-dir exists and is not a directory: %s", outputPath)
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return buildOutput{}, fmt.Errorf("inspect --output-dir: %w", statErr)
	}

	displayPath := outputPath
	if rel, ok := relativeWithin(rootAbs, outputPath); ok {
		displayPath = filepath.ToSlash(rel)
	}

	return buildOutput{
		AbsPath:     outputPath,
		DisplayPath: displayPath,
		Isolated:    true,
	}, nil
}

// resolveBuildPath resolves symlinks in the nearest existing ancestor and
// appends the components that do not exist yet. EvalSymlinks alone fails for a
// new child below an existing symlink, which would let a lexical path pass the
// source-content denylist while publication writes through the link.
func resolveBuildPath(path string) (string, error) {
	clean := filepath.Clean(path)
	missing := make([]string, 0, 4)
	current := clean

	for {
		_, err := os.Lstat(current)
		if err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", fmt.Errorf("resolve symlinks in %s: %w", current, err)
			}
			for _, m := range slices.Backward(missing) {
				resolved = filepath.Join(resolved, m)
			}
			return filepath.Clean(resolved), nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("inspect %s: %w", current, err)
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("cannot resolve %s: no existing ancestor", path)
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

// buildNeedsAssets reports whether the selected processes embed assets/dist.
// cmd/migrate does not import the assets package. Every other process is
// treated conservatively as asset-dependent, including application-specific
// processes whose import graph andurel cannot know without compiling them.
func buildNeedsAssets(selected []buildProcess) bool {
	for _, process := range selected {
		if process.Name != "migrate" {
			return true
		}
	}

	return false
}

func buildHasNodeModules(rootDir string) bool {
	info, err := os.Stat(filepath.Join(rootDir, "node_modules"))
	return err == nil && info.IsDir()
}

func buildUsesGoWork(rootDir string) bool {
	_, err := os.Stat(filepath.Join(rootDir, "go.work"))
	return err == nil
}

// buildGoEnv pins the Go workspace to the staged tree when one was copied.
// Without a go.work file, GOWORK=off keeps an unrelated parent workspace from
// leaking into an isolated build.
func buildGoEnv(rootDir string) []string {
	env := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "GOWORK=") {
			continue
		}
		env = append(env, entry)
	}

	workFile := filepath.Join(rootDir, "go.work")
	if _, err := os.Stat(workFile); err == nil {
		return append(env, "GOWORK="+workFile)
	}

	return append(env, "GOWORK=off")
}

// stageBuildSource copies the project into a private staging directory. The
// source tree is only read. Build tools write generated Go, Vite assets and
// email renderers inside the stage, so two builds of the same source tree with
// different output directories never share a writable path.
func stageBuildSource(sourceRoot, outputDir string) (string, error) {
	sourceRoot, err := filepath.Abs(sourceRoot)
	if err != nil {
		return "", fmt.Errorf("resolve project root: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(sourceRoot)
	if err != nil {
		return "", fmt.Errorf("resolve project root symlinks: %w", err)
	}
	sourceRoot = resolvedRoot

	stageRoot, err := os.MkdirTemp("", "andurel-build-")
	if err != nil {
		return "", fmt.Errorf("create build staging directory: %w", err)
	}

	cleanup := func() { _ = os.RemoveAll(stageRoot) }

	skipRoots := map[string]bool{
		".git":         true,
		"dist":         true,
		"node_modules": true,
	}
	if rel, ok := relativeWithin(sourceRoot, outputDir); ok {
		skipRoots[filepath.ToSlash(rel)] = true
	}

	if err := copyBuildTree(sourceRoot, stageRoot, skipRoots); err != nil {
		cleanup()
		return "", err
	}
	if err := cloneNodeModules(sourceRoot, stageRoot); err != nil {
		cleanup()
		return "", err
	}
	if err := rewriteStageWorkspace(sourceRoot, stageRoot); err != nil {
		cleanup()
		return "", err
	}

	return stageRoot, nil
}

func copyBuildTree(sourceRoot, stageRoot string, skipRoots map[string]bool) error {
	mappings := []buildStageMapping{{source: sourceRoot, stage: stageRoot}}
	return filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		for skip := range skipRoots {
			if relSlash == skip || strings.HasPrefix(relSlash, skip+"/") {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		target := filepath.Join(stageRoot, rel)
		return copyBuildEntry(path, target, entry, mappings)
	})
}

// buildStageMapping maps a source tree to the private copy that build tools
// may write. Symlink targets inside a mapped source tree are retargeted at the
// corresponding staged path so a tool cannot follow a link back into source.
type buildStageMapping struct {
	source string
	stage  string
}

func cloneNodeModules(sourceRoot, stageRoot string) error {
	source := filepath.Join(sourceRoot, "node_modules")
	info, err := os.Lstat(source)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("inspect node_modules: %w", err)
	}

	target := filepath.Join(stageRoot, "node_modules")
	if info.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(source)
		if err != nil {
			return fmt.Errorf("resolve node_modules symlink: %w", err)
		}
		source = resolved
		if info, err = os.Stat(source); err != nil {
			return fmt.Errorf("inspect node_modules target: %w", err)
		}
	}
	if !info.IsDir() {
		return fmt.Errorf("node_modules is not a directory: %s", source)
	}

	mappings := []buildStageMapping{
		{source: sourceRoot, stage: stageRoot},
		{source: source, stage: target},
	}
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(target, 0o755)
		}
		relSlash := filepath.ToSlash(rel)
		for _, cacheDir := range []string{".vite", ".vite-temp", ".cache"} {
			if relSlash == cacheDir || strings.HasPrefix(relSlash, cacheDir+"/") ||
				strings.Contains(relSlash, "/"+cacheDir+"/") ||
				strings.HasSuffix(relSlash, "/"+cacheDir) {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		return cloneBuildEntry(path, filepath.Join(target, rel), entry, mappings)
	})
}

func copyBuildEntry(
	source, target string,
	entry fs.DirEntry,
	mappings []buildStageMapping,
) error {
	if entry.IsDir() {
		return os.MkdirAll(target, 0o755)
	}

	info, err := entry.Info()
	if err != nil {
		return err
	}

	switch {
	case info.Mode()&os.ModeSymlink != 0:
		return stageBuildSymlink(source, target, mappings)
	case info.Mode().IsRegular():
		return copyBuildFile(source, target, info.Mode().Perm())
	default:
		return nil
	}
}

func cloneBuildEntry(
	source, target string,
	entry fs.DirEntry,
	mappings []buildStageMapping,
) error {
	if entry.IsDir() {
		return os.MkdirAll(target, 0o755)
	}

	info, err := entry.Info()
	if err != nil {
		return err
	}

	switch {
	case info.Mode()&os.ModeSymlink != 0:
		return stageBuildSymlink(source, target, mappings)
	case info.Mode().IsRegular():
		// A hardlink would share an inode with the source dependency. A build
		// plugin that rewrites the staged dependency would then silently modify
		// the project tree and every concurrent staged build.
		return copyBuildFile(source, target, info.Mode().Perm())
	default:
		return nil
	}
}

// stageBuildSymlink recreates a source symlink inside the stage. Targets that
// resolve into the project or the installed dependency tree are rewritten to
// their staged counterparts. A target outside those trees is rejected instead
// of copied verbatim: writes through it could not be kept private to the stage.
func stageBuildSymlink(sourceLink, target string, mappings []buildStageMapping) error {
	link, err := os.Readlink(sourceLink)
	if err != nil {
		return fmt.Errorf("read symlink %s: %w", sourceLink, err)
	}

	resolvedTarget := link
	if !filepath.IsAbs(resolvedTarget) {
		resolvedTarget = filepath.Join(filepath.Dir(sourceLink), resolvedTarget)
	}
	resolvedTarget = filepath.Clean(resolvedTarget)
	if resolved, resolveErr := filepath.EvalSymlinks(resolvedTarget); resolveErr == nil {
		resolvedTarget = resolved
	}

	for _, mapping := range mappings {
		stagedTarget, ok := stagedBuildPath(mapping, resolvedTarget)
		if !ok {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		relativeTarget, err := filepath.Rel(filepath.Dir(target), stagedTarget)
		if err != nil {
			relativeTarget = stagedTarget
		}
		if err := os.Symlink(relativeTarget, target); err != nil {
			return fmt.Errorf("stage symlink %s: %w", sourceLink, err)
		}
		return nil
	}

	return fmt.Errorf(
		"refusing symlink %s: target %s is outside the project and the dependency snapshot; an isolated build cannot keep writes through it private",
		sourceLink,
		resolvedTarget,
	)
}

func stagedBuildPath(mapping buildStageMapping, target string) (string, bool) {
	source := filepath.Clean(mapping.source)
	target = filepath.Clean(target)
	if target == source {
		return mapping.stage, true
	}
	rel, ok := relativeWithin(source, target)
	if !ok {
		return "", false
	}
	return filepath.Join(mapping.stage, rel), true
}

func copyBuildFile(source, target string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open %s: %w", source, err)
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("create %s: %w", target, err)
	}

	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("copy %s: %w", source, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close %s: %w", target, closeErr)
	}
	if err := os.Chmod(target, mode|0o200); err != nil {
		return fmt.Errorf("chmod %s: %w", target, err)
	}

	return nil
}

// rewriteStageWorkspace makes relative go.mod/go.work paths point at the staged
// tree or, for external local modules, at the original absolute location.
func rewriteStageWorkspace(sourceRoot, stageRoot string) error {
	for _, name := range []string{"go.mod", "go.work"} {
		path := filepath.Join(stageRoot, name)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("read staged %s: %w", name, err)
		}

		rewritten, changed, err := rewriteWorkspacePaths(
			path,
			data,
			sourceRoot,
			stageRoot,
			name == "go.work",
		)
		if err != nil {
			return fmt.Errorf("rewrite staged %s: %w", name, err)
		}
		if !changed {
			continue
		}
		if err := os.WriteFile(path, rewritten, 0o644); err != nil {
			return fmt.Errorf("write staged %s: %w", name, err)
		}
	}

	return nil
}

// rewriteWorkspacePaths uses the Go module-file parser so quoted filesystem
// paths, comments and block syntax survive the rewrite. A line splitter cannot
// represent replace/use paths that contain spaces.
func rewriteWorkspacePaths(
	path string,
	content []byte,
	sourceRoot, stageRoot string,
	workFile bool,
) ([]byte, bool, error) {
	if workFile {
		work, err := modfile.ParseWork(path, content, nil)
		if err != nil {
			return nil, false, err
		}

		changed := false
		for _, use := range work.Use {
			if !isRelativeFilesystemPath(use.Path) {
				continue
			}
			mapped := mapStagePath(use.Path, sourceRoot, stageRoot)
			if mapped == use.Path {
				continue
			}
			setWorkUsePath(use, mapped)
			changed = true
		}
		for _, replace := range work.Replace {
			if setStagedReplacePath(replace, sourceRoot, stageRoot) {
				changed = true
			}
		}
		if !changed {
			return content, false, nil
		}
		return modfile.Format(work.Syntax), true, nil
	}

	file, err := modfile.Parse(path, content, nil)
	if err != nil {
		return nil, false, err
	}

	changed := false
	for _, replace := range file.Replace {
		if setStagedReplacePath(replace, sourceRoot, stageRoot) {
			changed = true
		}
	}
	if !changed {
		return content, false, nil
	}
	return modfile.Format(file.Syntax), true, nil
}

func setWorkUsePath(use *modfile.Use, path string) {
	use.Path = path
	index := 0
	if len(use.Syntax.Token) > 1 && use.Syntax.Token[0] == "use" {
		index = 1
	}
	use.Syntax.Token[index] = modfile.AutoQuote(path)
}

func setStagedReplacePath(replace *modfile.Replace, sourceRoot, stageRoot string) bool {
	if replace.New.Version != "" || !isRelativeFilesystemPath(replace.New.Path) {
		return false
	}
	mapped := mapStagePath(replace.New.Path, sourceRoot, stageRoot)
	if mapped == replace.New.Path {
		return false
	}

	for index, token := range replace.Syntax.Token {
		if token != "=>" || index+1 >= len(replace.Syntax.Token) {
			continue
		}
		replace.New.Path = mapped
		replace.Syntax.Token[index+1] = modfile.AutoQuote(mapped)
		return true
	}
	return false
}

func isRelativeFilesystemPath(path string) bool {
	return path == "." || path == ".." ||
		strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../")
}

func mapStagePath(rel, sourceRoot, stageRoot string) string {
	abs := filepath.Clean(filepath.Join(sourceRoot, rel))
	if rel == "." {
		return stageRoot
	}
	if inner, ok := relativeWithin(sourceRoot, abs); ok {
		staged := filepath.Join(stageRoot, inner)
		if _, err := os.Stat(staged); err == nil {
			return staged
		}
	}
	return abs
}

func relativeWithin(root, path string) (string, bool) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", false
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

func isPathWithin(path, root string) bool {
	rel, err := filepath.Rel(path, root)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// Publication names. publishBuildArtifacts stages temporary outputs and
// backups with these prefixes. A journal makes recovery all-or-nothing across
// every selected process: a build killed between two renames must never pair a
// newly published app with the previous SSR binary.
const (
	buildTempPrefix   = ".andurel-"
	buildBackupPrefix = ".andurel-backup-"
	buildJournalName  = ".andurel-publish-journal.json"
	buildPublishing   = "publishing"
	buildCommitted    = "committed"
)

type buildPublicationJournalEntry struct {
	Process     string `json:"process"`
	Final       string `json:"final"`
	Temp        string `json:"temp"`
	Backup      string `json:"backup,omitempty"`
	HadPrevious bool   `json:"had_previous"`
}

type buildPublicationJournal struct {
	Phase   string                         `json:"phase"`
	Entries []buildPublicationJournalEntry `json:"entries"`
}

// recoverInterruptedBuildPublication repairs output left by a build that was
// killed mid-publication. A journal is authoritative for the whole artifact
// set and is rolled back or forward as a unit. Without a journal, backups from
// older andurel versions are repaired one file at a time (legacy recovery).
func recoverInterruptedBuildPublication(outputDir string) ([]string, error) {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read output directory: %w", err)
	}

	journalPath := filepath.Join(outputDir, buildJournalName)
	data, err := os.ReadFile(journalPath)
	switch {
	case err == nil:
		journal, decodeErr := decodeBuildPublicationJournal(data)
		if decodeErr != nil {
			return nil, fmt.Errorf("read interrupted build publication journal: %w", decodeErr)
		}
		recovered, recoverErr := recoverBuildPublicationJournal(outputDir, journal)
		if recoverErr != nil {
			return recovered, recoverErr
		}
		if removeErr := os.Remove(journalPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return recovered, fmt.Errorf("remove build publication journal: %w", removeErr)
		}
		removeBuildJournalTemps(outputDir)
		return recovered, nil
	case !os.IsNotExist(err):
		return nil, fmt.Errorf("read build publication journal: %w", err)
	}

	recovered := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		path := filepath.Join(outputDir, name)
		switch {
		case strings.HasPrefix(name, buildJournalName+".tmp-"):
			if err := os.Remove(path); err != nil {
				return recovered, fmt.Errorf("remove temporary build journal %s: %w", name, err)
			}
		case strings.HasPrefix(name, buildBackupPrefix):
			process, ok := buildPublicationProcess(strings.TrimPrefix(name, buildBackupPrefix))
			if !ok {
				continue
			}
			final := filepath.Join(outputDir, process)
			if _, statErr := os.Stat(final); statErr == nil {
				if err := os.Remove(path); err != nil {
					return recovered, fmt.Errorf("remove stale backup %s: %w", name, err)
				}
				recovered = append(recovered, "removed stale backup of "+process)
				continue
			}
			if err := os.Rename(path, final); err != nil {
				return recovered, fmt.Errorf("restore interrupted publication of %s: %w", process, err)
			}
			recovered = append(recovered, "restored interrupted publication of "+process)
		case strings.HasPrefix(name, buildTempPrefix):
			if _, ok := buildPublicationProcess(strings.TrimPrefix(name, buildTempPrefix)); !ok {
				continue
			}
			if err := os.Remove(path); err != nil {
				return recovered, fmt.Errorf("remove temporary output %s: %w", name, err)
			}
			recovered = append(recovered, "removed temporary output for "+strings.TrimPrefix(name, buildTempPrefix))
		}
	}
	return recovered, nil
}

func decodeBuildPublicationJournal(data []byte) (*buildPublicationJournal, error) {
	var journal buildPublicationJournal
	if err := json.Unmarshal(data, &journal); err != nil {
		return nil, err
	}
	if journal.Phase != buildPublishing && journal.Phase != buildCommitted {
		return nil, fmt.Errorf("unknown publication phase %q", journal.Phase)
	}
	if len(journal.Entries) == 0 {
		return nil, fmt.Errorf("publication journal has no entries")
	}

	seen := make(map[string]bool, len(journal.Entries))
	for _, entry := range journal.Entries {
		if !isSafeBuildPublicationName(entry.Process) || entry.Final != entry.Process {
			return nil, fmt.Errorf("unsafe publication process %q", entry.Process)
		}
		if seen[entry.Process] {
			return nil, fmt.Errorf("duplicate publication process %q", entry.Process)
		}
		seen[entry.Process] = true
		if !isBuildPublicationTempName(entry.Temp, entry.Process) {
			return nil, fmt.Errorf("unsafe temporary output %q", entry.Temp)
		}
		if entry.HadPrevious != (entry.Backup != "") {
			return nil, fmt.Errorf("inconsistent backup for process %q", entry.Process)
		}
		if entry.Backup != "" && !isBuildPublicationBackupName(entry.Backup, entry.Process) {
			return nil, fmt.Errorf("unsafe backup output %q", entry.Backup)
		}
	}
	return &journal, nil
}

func isSafeBuildPublicationName(name string) bool {
	return name != "" && name != "." && name != ".." &&
		filepath.Base(name) == name && !strings.ContainsAny(name, `/\`)
}

func isBuildPublicationTempName(name, process string) bool {
	if !strings.HasPrefix(name, buildTempPrefix) || !isSafeBuildPublicationName(name) {
		return false
	}
	parsed, ok := buildPublicationProcess(strings.TrimPrefix(name, buildTempPrefix))
	return ok && parsed == process
}

func isBuildPublicationBackupName(name, process string) bool {
	if !strings.HasPrefix(name, buildBackupPrefix) || !isSafeBuildPublicationName(name) {
		return false
	}
	parsed, ok := buildPublicationProcess(strings.TrimPrefix(name, buildBackupPrefix))
	return ok && parsed == process
}

func recoverBuildPublicationJournal(outputDir string, journal *buildPublicationJournal) ([]string, error) {
	switch journal.Phase {
	case buildPublishing:
		return rollbackBuildPublication(outputDir, journal)
	case buildCommitted:
		return finalizeBuildPublication(outputDir, journal)
	default:
		return nil, fmt.Errorf("unknown publication phase %q", journal.Phase)
	}
}

// rollbackBuildPublication restores the complete previous artifact set. An
// entry that had no previous artifact is removed if its rename completed.
func rollbackBuildPublication(outputDir string, journal *buildPublicationJournal) ([]string, error) {
	recovered := make([]string, 0, len(journal.Entries))
	for _, entry := range journal.Entries {
		final := filepath.Join(outputDir, entry.Final)
		temp := filepath.Join(outputDir, entry.Temp)

		if entry.Backup != "" {
			backup := filepath.Join(outputDir, entry.Backup)
			if _, err := os.Lstat(backup); err == nil {
				if err := os.Remove(final); err != nil && !os.IsNotExist(err) {
					return recovered, fmt.Errorf("remove partially published %s: %w", entry.Process, err)
				}
				if err := os.Rename(backup, final); err != nil {
					return recovered, fmt.Errorf("restore previous %s: %w", entry.Process, err)
				}
				recovered = append(recovered, "restored previous "+entry.Process)
			} else if !os.IsNotExist(err) {
				return recovered, fmt.Errorf("inspect backup for %s: %w", entry.Process, err)
			}
		} else if !entry.HadPrevious {
			if _, err := os.Lstat(temp); os.IsNotExist(err) {
				if _, finalErr := os.Lstat(final); finalErr == nil {
					if err := os.Remove(final); err != nil {
						return recovered, fmt.Errorf("remove interrupted publication of %s: %w", entry.Process, err)
					}
					recovered = append(recovered, "removed interrupted publication of "+entry.Process)
				} else if !os.IsNotExist(finalErr) {
					return recovered, fmt.Errorf("inspect partial %s: %w", entry.Process, finalErr)
				}
			}
		}

		if err := os.Remove(temp); err != nil && !os.IsNotExist(err) {
			return recovered, fmt.Errorf("remove temporary output for %s: %w", entry.Process, err)
		}
	}
	return recovered, nil
}

// finalizeBuildPublication completes a publication whose journal reached the
// committed phase. The new artifact set is kept and only backups are removed.
func finalizeBuildPublication(outputDir string, journal *buildPublicationJournal) ([]string, error) {
	recovered := make([]string, 0, len(journal.Entries))
	for _, entry := range journal.Entries {
		final := filepath.Join(outputDir, entry.Final)
		temp := filepath.Join(outputDir, entry.Temp)

		if _, err := os.Lstat(final); err != nil {
			if !os.IsNotExist(err) {
				return recovered, fmt.Errorf("inspect published %s: %w", entry.Process, err)
			}
			if _, tempErr := os.Lstat(temp); tempErr == nil {
				if err := os.Rename(temp, final); err != nil {
					return recovered, fmt.Errorf("complete publication of %s: %w", entry.Process, err)
				}
				recovered = append(recovered, "completed publication of "+entry.Process)
			} else if !os.IsNotExist(tempErr) {
				return recovered, fmt.Errorf("inspect temporary output for %s: %w", entry.Process, tempErr)
			} else {
				return recovered, fmt.Errorf("committed publication is missing %s", entry.Process)
			}
		}

		if entry.Backup != "" {
			backup := filepath.Join(outputDir, entry.Backup)
			if err := os.Remove(backup); err != nil && !os.IsNotExist(err) {
				return recovered, fmt.Errorf("remove stale backup for %s: %w", entry.Process, err)
			}
		}
		if err := os.Remove(temp); err != nil && !os.IsNotExist(err) {
			return recovered, fmt.Errorf("remove temporary output for %s: %w", entry.Process, err)
		}
	}
	return recovered, nil
}

func writeBuildPublicationJournal(outputDir string, journal *buildPublicationJournal) error {
	data, err := json.Marshal(journal)
	if err != nil {
		return fmt.Errorf("encode publication journal: %w", err)
	}
	temp, err := os.CreateTemp(outputDir, buildJournalName+".tmp-*")
	if err != nil {
		return fmt.Errorf("create publication journal: %w", err)
	}
	tempPath := temp.Name()
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		_ = os.Remove(tempPath)
		return fmt.Errorf("write publication journal: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		_ = os.Remove(tempPath)
		return fmt.Errorf("sync publication journal: %w", err)
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("close publication journal: %w", err)
	}
	if err := os.Rename(tempPath, filepath.Join(outputDir, buildJournalName)); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("publish publication journal: %w", err)
	}
	return nil
}

func removeBuildJournalTemps(outputDir string) {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), buildJournalName+".tmp-") {
			continue
		}
		_ = os.Remove(filepath.Join(outputDir, entry.Name()))
	}
}

// buildPublicationProcess extracts the process name from a CreateTemp suffix
// such as "app-123456789". The random part is all digits; the process name may
// itself contain dashes. Unsafe path-shaped names are ignored.
func buildPublicationProcess(suffix string) (string, bool) {
	dash := strings.LastIndex(suffix, "-")
	if dash <= 0 || dash == len(suffix)-1 {
		return "", false
	}
	random := suffix[dash+1:]
	for index := 0; index < len(random); index++ {
		if random[index] < '0' || random[index] > '9' {
			return "", false
		}
	}
	process := suffix[:dash]
	if process == "." || process == ".." || strings.ContainsAny(process, `/\\`) {
		return "", false
	}
	return process, true
}

// publishBuildArtifacts copies staged binaries into outputDir with an
// all-or-nothing publication. A journal records every selected process before
// the first rename; a failed copy or rename leaves the previous output intact,
// and recovery rolls the whole artifact set back or forward. No success
// manifest is produced for a failed publication.
func publishBuildArtifacts(
	workRoot, outputDir string,
	selected []buildProcess,
	report *buildReport,
) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	recovered, err := recoverInterruptedBuildPublication(outputDir)
	if err != nil {
		return err
	}
	if len(recovered) > 0 && report != nil {
		report.Recovered = recovered
	}

	type publication struct {
		process     string
		final       string
		temp        string
		backup      string
		size        int64
		hadPrevious bool
	}
	publications := make([]publication, 0, len(selected))

	cleanupTemps := func() {
		for _, item := range publications {
			if item.temp != "" {
				_ = os.Remove(item.temp)
			}
		}
	}

	for _, process := range selected {
		staged := filepath.Join(workRoot, "dist", process.Name)
		info, err := os.Stat(staged)
		if err != nil {
			cleanupTemps()
			return fmt.Errorf("stat staged %s: %w", process.Name, err)
		}

		temp, err := os.CreateTemp(outputDir, buildTempPrefix+process.Name+"-*")
		if err != nil {
			cleanupTemps()
			return fmt.Errorf("create temporary output for %s: %w", process.Name, err)
		}
		tempPath := temp.Name()
		if err := temp.Close(); err != nil {
			_ = os.Remove(tempPath)
			cleanupTemps()
			return fmt.Errorf("close temporary output for %s: %w", process.Name, err)
		}
		if err := copyBuildFile(staged, tempPath, 0o755); err != nil {
			_ = os.Remove(tempPath)
			cleanupTemps()
			return fmt.Errorf("stage output for %s: %w", process.Name, err)
		}

		publications = append(publications, publication{
			process: process.Name,
			final:   filepath.Join(outputDir, process.Name),
			temp:    tempPath,
			size:    info.Size(),
		})
	}

	journal := &buildPublicationJournal{Phase: buildPublishing}
	for index := range publications {
		item := &publications[index]
		if _, err := os.Stat(item.final); err == nil {
			item.hadPrevious = true
			backup, err := os.CreateTemp(outputDir, buildBackupPrefix+item.process+"-*")
			if err != nil {
				cleanupTemps()
				return fmt.Errorf("prepare backup for %s: %w", item.process, err)
			}
			item.backup = backup.Name()
			if err := backup.Close(); err != nil {
				_ = os.Remove(item.backup)
				cleanupTemps()
				return fmt.Errorf("close backup for %s: %w", item.process, err)
			}
			if err := os.Remove(item.backup); err != nil {
				cleanupTemps()
				return fmt.Errorf("remove backup placeholder for %s: %w", item.process, err)
			}
		} else if !os.IsNotExist(err) {
			cleanupTemps()
			return fmt.Errorf("inspect previous %s: %w", item.process, err)
		}

		entry := buildPublicationJournalEntry{
			Process:     item.process,
			Final:       item.process,
			Temp:        filepath.Base(item.temp),
			HadPrevious: item.hadPrevious,
		}
		if item.backup != "" {
			entry.Backup = filepath.Base(item.backup)
		}
		journal.Entries = append(journal.Entries, entry)
	}

	if err := writeBuildPublicationJournal(outputDir, journal); err != nil {
		cleanupTemps()
		return err
	}

	rollback := func(cause error) error {
		_, rollbackErr := rollbackBuildPublication(outputDir, journal)
		if rollbackErr == nil {
			_ = os.Remove(filepath.Join(outputDir, buildJournalName))
		}
		if rollbackErr != nil {
			return fmt.Errorf("%w; rollback failed: %v", cause, rollbackErr)
		}
		return cause
	}

	for index := range publications {
		item := &publications[index]
		if item.backup == "" {
			continue
		}
		if err := os.Rename(item.final, item.backup); err != nil {
			return rollback(fmt.Errorf("back up previous %s: %w", item.process, err))
		}
	}

	for index := range publications {
		item := &publications[index]
		if err := os.Rename(item.temp, item.final); err != nil {
			return rollback(fmt.Errorf("publish %s: %w", item.process, err))
		}
		item.temp = ""
	}

	journal.Phase = buildCommitted
	if err := writeBuildPublicationJournal(outputDir, journal); err != nil {
		return rollback(fmt.Errorf("commit build publication: %w", err))
	}

	for _, item := range publications {
		if item.backup != "" {
			_ = os.Remove(item.backup)
		}
	}
	_ = os.Remove(filepath.Join(outputDir, buildJournalName))
	removeBuildJournalTemps(outputDir)

	if report != nil {
		for index := range report.Artifacts {
			artifact := &report.Artifacts[index]
			for _, item := range publications {
				if artifact.Process != item.process {
					continue
				}
				artifact.Path = filepath.ToSlash(item.final)
				artifact.Size = item.size
				break
			}
		}
		report.OutputDir = filepath.ToSlash(outputDir)
	}

	return nil
}
