package layers

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"
)

// StorageImportPath is the pkg/storage module path. It is a separate Go module
// from the CLI, so it cannot be derived from the project's module path.
const StorageImportPath = "github.com/mbvlabs/andurel/pkg/storage"

// Violation is one layer finding in the verify output shape.
type Violation struct {
	Rule    string `json:"rule"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	Message string `json:"message"`
	Fix     string `json:"fix,omitempty"`
}

// Package is one loaded project package with the type information analyzers
// need. Dependencies are intentionally not loaded with syntax; their export
// data is enough for type identity.
type Package struct {
	ID              string
	Path            string
	Name            string
	Dir             string
	ForTest         string
	Files           []string
	CompiledGoFiles []string
	Syntax          []*ast.File
	Types           *types.Package
	TypesInfo       *types.Info
	TypesSizes      types.Sizes
}

// Project is a loaded Go project rooted at a directory containing go.mod.
// W5 can reuse it to inspect query instantiations without loading the project
// again.
type Project struct {
	RootDir  string
	Module   string
	Fset     *token.FileSet
	Packages []*Package

	funcDecls    map[token.Pos]*ast.FuncDecl
	funcPackages map[*ast.FuncDecl]*Package
	packageTypes map[*types.Package]*Package
}

// noPackagesError explains an empty load. go/packages reports no packages and
// no error when the go command refuses to run, for example when go.mod needs
// tidying, so the go command's own message is recovered by running go list.
func noPackagesError(rootDir string, patterns []string) error {
	args := append([]string{"list"}, patterns...)
	cmd := exec.Command("go", args...)
	cmd.Dir = rootDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		if message := strings.TrimSpace(string(output)); message != "" {
			return fmt.Errorf("load Go packages: %s", message)
		}
	}

	return errors.New("no Go packages found under the project root")
}

// Load loads rootDir and everything under it with the minimal type-checking
// mode the layer analyzers need. Tests are included so the checks do not
// silently narrow to production files.
//
// The load follows the active build target, as go/packages and the go command
// do: files excluded by GOOS, GOARCH, or build constraints are not parsed,
// type-checked, or analyzed. This keeps verify honest for the code it actually
// builds without duplicating the toolchain's cross-platform selection.
func Load(rootDir string) (*Project, error) {
	return LoadPatterns(rootDir, nil)
}

// LoadPatterns loads the packages matched by patterns. An empty pattern list
// means the whole project, matching Load. Patterns use the go command's package
// syntax (for example ./app/...), so a scoped verify checks exactly
// the packages it was asked about.
func LoadPatterns(rootDir string, patterns []string) (*Project, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolve project root: %w", err)
	}
	module, err := readModulePath(absRoot)
	if err != nil {
		return nil, err
	}
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}

	fset := token.NewFileSet()
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedImports |
			packages.NeedTypesSizes,
		Dir:   absRoot,
		Fset:  fset,
		Tests: true,
	}
	loaded, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, fmt.Errorf("load Go packages: %w", err)
	}
	if len(loaded) == 0 {
		return nil, noPackagesError(absRoot, patterns)
	}

	// Load query syntax in the same type-checking graph without expanding scope.
	scopedIDs := make(map[string]bool, len(loaded))
	needsQueries, hasQueries := false, false
	queryPath := module + "/domains/internal/queries"
	for _, pkg := range loaded {
		scopedIDs[pkg.ID] = true
		hasQueries = hasQueries || pkg.PkgPath == queryPath
		_, importsQueries := pkg.Imports[queryPath]
		needsQueries = needsQueries || importsQueries
	}
	if needsQueries && !hasQueries {
		loaded, err = packages.Load(cfg, append(append([]string{}, patterns...), queryPath)...)
		if err != nil {
			return nil, fmt.Errorf("load Go packages: %w", err)
		}
	}

	loadErrors := make([]error, 0)
	for _, pkg := range loaded {
		for _, pkgErr := range pkg.Errors {
			loadErrors = append(loadErrors, fmt.Errorf("%s: %s", pkgErr.Pos, pkgErr.Msg))
		}
	}
	if len(loadErrors) > 0 {
		return nil, fmt.Errorf("type-check project: %w", errors.Join(loadErrors...))
	}

	buildCache, hasBuildCache := testMainCacheDir(absRoot, loaded, goEnvValue)
	project := &Project{
		RootDir:      absRoot,
		Module:       module,
		Fset:         fset,
		Packages:     make([]*Package, 0, len(loaded)),
		funcDecls:    make(map[token.Pos]*ast.FuncDecl),
		funcPackages: make(map[*ast.FuncDecl]*Package),
		packageTypes: make(map[*types.Package]*Package),
	}
	for _, pkg := range loaded {
		if hasBuildCache && isSyntheticTestMain(pkg, buildCache) {
			continue
		}
		if pkg.Types == nil {
			return nil, fmt.Errorf("package %s has no type information", pkg.PkgPath)
		}
		if len(pkg.Syntax) == 0 {
			continue
		}
		loadedPkg := &Package{
			ID:              pkg.ID,
			Path:            pkg.PkgPath,
			Name:            pkg.Name,
			Dir:             pkg.Dir,
			ForTest:         pkg.ForTest,
			Files:           pkg.GoFiles,
			CompiledGoFiles: pkg.CompiledGoFiles,
			Syntax:          pkg.Syntax,
			Types:           pkg.Types,
			TypesInfo:       pkg.TypesInfo,
			TypesSizes:      pkg.TypesSizes,
		}
		if scopedIDs[pkg.ID] {
			project.Packages = append(project.Packages, loadedPkg)
		}
		project.packageTypes[pkg.Types] = loadedPkg
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				project.funcDecls[fn.Name.Pos()] = fn
				project.funcPackages[fn] = loadedPkg
			}
		}
	}
	if len(project.Packages) == 0 {
		return nil, errors.New("no analyzable Go packages found under the project root")
	}
	return project, nil
}

const goTestMainHeader = "// Code generated by 'go test'. DO NOT EDIT."

// isTestMainCandidate narrows the go/packages identity of a generated test
// main before any cache or source inspection. Test mains do not reliably
// populate ForTest, so package name, path suffix and the single generated
// source are used instead.
func isTestMainCandidate(pkg *packages.Package) bool {
	return pkg.Name == "main" &&
		strings.HasSuffix(pkg.PkgPath, ".test") &&
		len(pkg.GoFiles) == 1
}

type goEnvFunc func(rootDir, name string) (string, error)

func goEnvValue(rootDir, name string) (string, error) {
	cmd := exec.Command("go", "env", name)
	cmd.Dir = rootDir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// testMainCacheDir asks the go command for the active build cache only when a
// test-main candidate exists, so a custom GOCACHE, including one inside the
// project root, is honored without adding work to ordinary loads. The lookup
// seam keeps failure and empty/off handling testable without running go.
func testMainCacheDir(rootDir string, loaded []*packages.Package, lookup goEnvFunc) (string, bool) {
	for _, pkg := range loaded {
		if !isTestMainCandidate(pkg) {
			continue
		}
		cacheDir, err := lookup(rootDir, "GOCACHE")
		cacheDir = strings.TrimSpace(cacheDir)
		if err != nil || cacheDir == "" || cacheDir == "off" {
			return "", false
		}
		return cacheDir, true
	}
	return "", false
}

// isSyntheticTestMain identifies the main package the go command generates to
// run a package's tests. The predicate is conservative: it combines the
// go/packages identity, the go command's actual cache data-artifact layout and
// the exact generated header. Cache containment alone is not enough because
// GOCACHE may be the project root or an ancestor, where an ordinary .test main
// with a generated-looking header lives inside the cache directory without
// being a cache artifact.
func isSyntheticTestMain(pkg *packages.Package, buildCache string) bool {
	if buildCache == "" || !isTestMainCandidate(pkg) || !isGoCacheDataFile(buildCache, pkg.GoFiles[0]) {
		return false
	}
	data, err := os.ReadFile(pkg.GoFiles[0])
	return err == nil && strings.HasPrefix(strings.TrimSpace(string(data)), goTestMainHeader)
}

// isGoCacheDataFile reports whether path has the go command's build-cache
// data-artifact layout: <cache>/<first two hex digits>/<64-hex action ID>-d.
// Ordinary project source, including .go files with generated headers, does not
// match even when GOCACHE is the project root or one of its ancestors.
func isGoCacheDataFile(cacheDir, path string) bool {
	rel, ok := relativeWithinDir(cacheDir, path)
	if !ok {
		return false
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) != 2 {
		return false
	}
	prefix, name := parts[0], parts[1]
	if len(prefix) != 2 || !isLowerHex(prefix) {
		return false
	}
	actionID, ok := strings.CutSuffix(name, "-d")
	if !ok || len(actionID) != 64 || !isLowerHex(actionID) {
		return false
	}
	return strings.HasPrefix(actionID, prefix)
}

func isLowerHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return s != ""
}

// relativeWithinDir returns path relative to dir when path is inside it. It
// prefers fully resolved paths: an artifact symlinked out of the cache must not
// look contained merely because its link name is inside the cache. Lexical
// comparison is only the fallback when resolution is unavailable.
func relativeWithinDir(dir, path string) (string, bool) {
	if realDir, err := filepath.EvalSymlinks(dir); err == nil {
		if realPath, err := filepath.EvalSymlinks(path); err == nil {
			return relWithin(realDir, realPath)
		}
	}
	return relWithin(dir, path)
}

func relWithin(dir, path string) (string, bool) {
	rel, err := filepath.Rel(dir, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

func pathWithinDir(dir, path string) bool {
	_, ok := relativeWithinDir(dir, path)
	return ok
}

// Analyze loads the project and runs every typed layer rule.
func Analyze(rootDir string) ([]Violation, error) {
	return AnalyzePatterns(rootDir, nil)
}

// AnalyzePatterns runs every typed layer rule over the packages matched by
// patterns. An empty pattern list means the whole project.
func AnalyzePatterns(rootDir string, patterns []string) ([]Violation, error) {
	return AnalyzePatternsRules(rootDir, patterns, nil)
}

// AnalyzePatternsRules runs only the named typed layer rules (catalog IDs)
// over the packages matched by patterns. An empty rule list runs every rule.
func AnalyzePatternsRules(
	rootDir string,
	patterns []string,
	ruleIDs []string,
) ([]Violation, error) {
	project, err := LoadPatterns(rootDir, patterns)
	if err != nil {
		return nil, err
	}
	return project.RunRules(ruleIDs)
}

// Run executes the layer analyzers over the loaded packages and returns
// deterministic, de-duplicated violations.
func (p *Project) Run() ([]Violation, error) {
	return p.RunRules(nil)
}

// RunRules executes only the analyzers whose catalog rule ID is in ruleIDs.
// An empty list runs every analyzer.
func (p *Project) RunRules(ruleIDs []string) ([]Violation, error) {
	cfg := Config{
		RootDir: p.RootDir,
		Module:  p.Module,
		Fset:    p.Fset,
		Project: p,
	}
	analyzers := Analyzers(cfg)
	if len(ruleIDs) > 0 {
		selected := make([]*analysis.Analyzer, 0, len(analyzers))
		for _, analyzer := range analyzers {
			if slices.Contains(ruleIDs, ruleIDForAnalyzer(analyzer)) {
				selected = append(selected, analyzer)
			}
		}
		analyzers = selected
	}

	violations := make([]Violation, 0)
	for _, pkg := range p.Packages {
		for _, analyzer := range analyzers {
			found, err := runAnalyzer(analyzer, pkg, p.Fset, cfg)
			if err != nil {
				return nil, fmt.Errorf("run %s on %s: %w", analyzer.Name, pkg.Path, err)
			}
			violations = append(violations, found...)
		}
	}
	return normalizeViolations(violations), nil
}

func (p *Project) funcDeclAt(pos token.Pos) (*ast.FuncDecl, *Package, bool) {
	fn, ok := p.funcDecls[pos]
	if !ok {
		return nil, nil, false
	}
	return fn, p.funcPackages[fn], true
}

func readModulePath(rootDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(rootDir, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	module := modfile.ModulePath(data)
	if module == "" {
		return "", errors.New("go.mod has no module directive")
	}
	return module, nil
}

func normalizeViolations(violations []Violation) []Violation {
	seen := make(map[string]bool, len(violations))
	unique := make([]Violation, 0, len(violations))
	for _, violation := range violations {
		key := fmt.Sprintf(
			"%s\x00%s\x00%d\x00%s\x00%s",
			violation.Rule,
			violation.File,
			violation.Line,
			violation.Message,
			violation.Fix,
		)
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, violation)
	}
	sort.SliceStable(unique, func(i, j int) bool {
		left, right := unique[i], unique[j]
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		if left.Rule != right.Rule {
			return left.Rule < right.Rule
		}
		return left.Message < right.Message
	})
	return unique
}

// layerDir returns the package's directory relative to the module path.
// Test variants keep the same source directory as the package they exercise,
// so an external test package in services/ is classified as services. A real
// production directory named services_test/ stays services_test.
func (p *Package) layerDir(module, rootDir string) string {
	if p.ForTest != "" {
		return layerDirFromPath(module, p.ForTest)
	}
	if p.Dir != "" && rootDir != "" {
		rel, err := filepath.Rel(rootDir, p.Dir)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			if rel == "." {
				return ""
			}
			return filepath.ToSlash(rel)
		}
	}
	return layerDirFromPath(module, p.Path)
}

// layerDirFromPath is the fallback for analysistest and synthetic packages
// that have no filesystem metadata. It does not strip a "_test" suffix: that
// would turn a real production services_test directory into services.
func layerDirFromPath(module, pkgPath string) string {
	if before, _, ok := strings.Cut(pkgPath, " ["); ok {
		pkgPath = before
	}
	if pkgPath == module {
		return ""
	}
	if rest, ok := strings.CutPrefix(pkgPath, module+"/"); ok {
		return rest
	}
	return pkgPath
}

func inDir(dir, want string) bool {
	return dir == want || strings.HasPrefix(dir, want+"/")
}

func inPersistenceDir(dir string) bool {
	return inDir(dir, "domains") ||
		inDir(dir, "app") ||
		inDir(dir, "cmd") ||
		inDir(dir, "seeds") ||
		inDir(dir, "migrations")
}
