package cli

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	if resolved, resolveErr := filepath.EvalSymlinks(rootAbs); resolveErr == nil {
		rootAbs = resolved
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
	if resolved, resolveErr := filepath.EvalSymlinks(outputPath); resolveErr == nil {
		outputPath = resolved
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
		return copyBuildEntry(path, target, entry)
	})
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

		return cloneBuildEntry(path, filepath.Join(target, rel), entry)
	})
}

func copyBuildEntry(source, target string, entry fs.DirEntry) error {
	if entry.IsDir() {
		return os.MkdirAll(target, 0o755)
	}

	info, err := entry.Info()
	if err != nil {
		return err
	}

	switch {
	case info.Mode()&os.ModeSymlink != 0:
		link, err := os.Readlink(source)
		if err != nil {
			return fmt.Errorf("read symlink %s: %w", source, err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.Symlink(link, target)
	case info.Mode().IsRegular():
		return copyBuildFile(source, target, info.Mode().Perm())
	default:
		return nil
	}
}

func cloneBuildEntry(source, target string, entry fs.DirEntry) error {
	if entry.IsDir() {
		return os.MkdirAll(target, 0o755)
	}

	info, err := entry.Info()
	if err != nil {
		return err
	}

	switch {
	case info.Mode()&os.ModeSymlink != 0:
		link, err := os.Readlink(source)
		if err != nil {
			return fmt.Errorf("read symlink %s: %w", source, err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.Symlink(link, target)
	case info.Mode().IsRegular():
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.Link(source, target); err == nil {
			return nil
		}
		return copyBuildFile(source, target, info.Mode().Perm())
	default:
		return nil
	}
}

func copyBuildFile(source, target string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open %s: %w", source, err)
	}
	defer in.Close()

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

		rewritten := rewriteWorkspacePaths(string(data), sourceRoot, stageRoot, name == "go.work")
		if rewritten == string(data) {
			continue
		}
		if err := os.WriteFile(path, []byte(rewritten), 0o644); err != nil {
			return fmt.Errorf("write staged %s: %w", name, err)
		}
	}

	return nil
}

func rewriteWorkspacePaths(content, sourceRoot, stageRoot string, workFile bool) string {
	lines := strings.SplitAfter(content, "\n")
	inUseBlock := false

	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if workFile {
			if inUseBlock {
				if strings.HasPrefix(trimmed, ")") {
					inUseBlock = false
					continue
				}
				lines[index] = rewriteUseLine(line, "", sourceRoot, stageRoot)
				continue
			}
			if strings.HasPrefix(trimmed, "use (") {
				inUseBlock = true
				continue
			}
			if strings.HasPrefix(trimmed, "use ") {
				lines[index] = rewriteUseLine(line, "use ", sourceRoot, stageRoot)
				continue
			}
		}
		if strings.Contains(line, "=>") {
			lines[index] = rewriteReplaceLine(line, sourceRoot, stageRoot)
		}
	}

	return strings.Join(lines, "")
}

func rewriteUseLine(line, prefix, sourceRoot, stageRoot string) string {
	indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	body := strings.TrimSpace(line)
	comment := ""
	if index := strings.Index(body, "//"); index >= 0 {
		comment = " " + strings.TrimSpace(body[index:])
		body = strings.TrimSpace(body[:index])
	}
	if body == "" {
		return line
	}
	path := strings.TrimSpace(strings.TrimPrefix(body, prefix))
	if path == "" {
		return line
	}
	mapped := mapStagePath(path, sourceRoot, stageRoot)
	if mapped == path {
		return line
	}
	return indent + prefix + quoteWorkspacePath(mapped) + comment + "\n"
}

func rewriteReplaceLine(line, sourceRoot, stageRoot string) string {
	index := strings.Index(line, "=>")
	if index < 0 {
		return line
	}
	right := line[index+2:]
	trailingNewline := ""
	if before, ok := strings.CutSuffix(right, "\n"); ok {
		right = before
		trailingNewline = "\n"
	}
	comment := ""
	if commentIndex := strings.Index(right, "//"); commentIndex >= 0 {
		comment = " " + strings.TrimSpace(right[commentIndex:])
		right = right[:commentIndex]
	}
	fields := strings.Fields(right)
	if len(fields) == 0 {
		return line
	}
	path := strings.Trim(fields[0], `"`)
	if !isRelativeFilesystemPath(path) {
		return line
	}
	mapped := mapStagePath(path, sourceRoot, stageRoot)
	if mapped == path {
		return line
	}
	return line[:index+2] + " " + quoteWorkspacePath(mapped) + comment + trailingNewline
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

func quoteWorkspacePath(path string) string {
	if strings.ContainsAny(path, " \t") {
		return strconv.Quote(path)
	}
	return path
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

// Publication names. publishBuildArtifacts creates temporary outputs and
// backups with these prefixes; a build killed between renames leaves them
// behind, so the next publication repairs them before touching output.
const (
	buildTempPrefix   = ".andurel-"
	buildBackupPrefix = ".andurel-backup-"
)

// recoverInterruptedBuildPublication repairs output left by a build that was
// killed mid-publication. A backup without its final file is the previous good
// binary: restore it instead of leaving the output missing. A backup beside a
// final file is stale and removed. Temporary outputs never became visible, so
// they are removed. Only names this package creates are touched.
func recoverInterruptedBuildPublication(outputDir string) ([]string, error) {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read output directory: %w", err)
	}
	recovered := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		path := filepath.Join(outputDir, name)
		switch {
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
// all-or-nothing publication. A failed copy or rename leaves the previous
// output untouched and never produces a success manifest.
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
		process   string
		final     string
		temp      string
		backup    string
		size      int64
		published bool
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

		temp, err := os.CreateTemp(outputDir, ".andurel-"+process.Name+"-*")
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

	rollback := func() {
		for _, item := range publications {
			if item.backup != "" {
				_ = os.Remove(item.final)
				_ = os.Rename(item.backup, item.final)
				continue
			}
			if item.published {
				_ = os.Remove(item.final)
			}
		}
	}

	for index := range publications {
		item := &publications[index]
		if _, err := os.Stat(item.final); err == nil {
			backup, err := os.CreateTemp(outputDir, ".andurel-backup-"+item.process+"-*")
			if err != nil {
				rollback()
				cleanupTemps()
				return fmt.Errorf("prepare backup for %s: %w", item.process, err)
			}
			backupPath := backup.Name()
			if err := backup.Close(); err != nil {
				_ = os.Remove(backupPath)
				rollback()
				cleanupTemps()
				return fmt.Errorf("close backup for %s: %w", item.process, err)
			}
			if err := os.Remove(backupPath); err != nil {
				rollback()
				cleanupTemps()
				return fmt.Errorf("remove backup placeholder for %s: %w", item.process, err)
			}
			if err := os.Rename(item.final, backupPath); err != nil {
				rollback()
				cleanupTemps()
				return fmt.Errorf("back up previous %s: %w", item.process, err)
			}
			item.backup = backupPath
		}
	}

	for index := range publications {
		item := &publications[index]
		if err := os.Rename(item.temp, item.final); err != nil {
			rollback()
			cleanupTemps()
			return fmt.Errorf("publish %s: %w", item.process, err)
		}
		item.temp = ""
		item.published = true
	}

	for _, item := range publications {
		if item.backup != "" {
			_ = os.Remove(item.backup)
		}
	}

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

	return nil
}
