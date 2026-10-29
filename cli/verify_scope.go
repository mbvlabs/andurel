package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mbvlabs/andurel/v2/cli/internal/content"
	"github.com/mbvlabs/andurel/v2/cli/internal/layers"
	"github.com/mbvlabs/andurel/v2/cli/output"
)

// verifyStageSpec is one selectable verify stage. ID is the value --only
// accepts; RuleIDs are the catalog rules the stage executes. A stage without
// RuleIDs is selected only by --only.
type verifyStageSpec struct {
	ID       string
	StepName string
	RuleIDs  []string
}

// Stage IDs. They are the values --only accepts and the keys in
// verifyReport.Scope.Stages.
const (
	verifyStageSetup     = "setup"
	verifyStageGofmt     = "gofmt"
	verifyStageGoFix     = "go-fix"
	verifyStageGoVet     = "go-vet"
	verifyStageLayers    = "layers"
	verifyStageFrontend  = "frontend"
	verifyStageGenerated = "generated"
	verifyStageContent   = "content"
	verifyStageGoTest    = "go-test"
)

// contentRuleIDs are the rules the content stage reports. They live in the
// layer catalog so `verify --help` and `commands --json` advertise them.
var contentRuleIDs = []string{
	"content-config",
	"content-catalog",
	"content-slug",
	"content-required",
	"content-link",
	"content-checker",
}

func layerRuleIDs() []string {
	rules := layers.LayerRules()
	ids := make([]string, 0, len(rules))
	for _, rule := range rules {
		ids = append(ids, rule.ID)
	}
	return ids
}

// verifyStages returns the ordered selectable stages.
func verifyStages() []verifyStageSpec {
	return []verifyStageSpec{
		{ID: verifyStageSetup, StepName: "setup", RuleIDs: []string{"setup", "missing-tool"}},
		{ID: verifyStageGofmt, StepName: "gofmt", RuleIDs: []string{"gofmt"}},
		{ID: verifyStageGoFix, StepName: "go fix", RuleIDs: []string{"go fix"}},
		{ID: verifyStageGoVet, StepName: "go vet"},
		{ID: verifyStageLayers, StepName: "layers", RuleIDs: layerRuleIDs()},
		{
			ID:       verifyStageFrontend,
			StepName: "frontend",
			RuleIDs:  []string{"frontend", "frontend-route-usage"},
		},
		{ID: verifyStageGenerated, StepName: "generated", RuleIDs: []string{"generated"}},
		{ID: verifyStageContent, StepName: "content", RuleIDs: contentRuleIDs},
		{ID: verifyStageGoTest, StepName: "go test"},
	}
}

func verifyStageByID(id string) (verifyStageSpec, bool) {
	for _, stage := range verifyStages() {
		if stage.ID == id {
			return stage, true
		}
	}
	return verifyStageSpec{}, false
}

func verifyStageIDs() []string {
	stages := verifyStages()
	ids := make([]string, 0, len(stages))
	for _, stage := range stages {
		ids = append(ids, stage.ID)
	}
	return ids
}

// verifySelection records the stages and rules a run selected.
type verifySelection struct {
	StageIDs []string
	RuleIDs  []string
	// ruleFiltered is true when --rules restricted the rules executed inside
	// the selected stages.
	ruleFiltered bool
}

// parseVerifySelection validates --only and --rules. Unknown names are usage
// errors; an empty name is never silently ignored.
func parseVerifySelection(only, rules []string) (verifySelection, error) {
	selection := verifySelection{}
	only = splitVerifyNames(only)
	rules = splitVerifyNames(rules)
	selection.ruleFiltered = len(rules) > 0
	stageSet := map[string]bool{}
	for _, id := range only {
		if _, ok := verifyStageByID(id); !ok {
			return selection, output.NewError(
				output.CodeUsage,
				fmt.Sprintf("unknown verify stage %q", id),
				output.ExitUsage,
				"Use one of: "+strings.Join(verifyStageIDs(), ", ")+". Stages are also listed in andurel verify --help.",
			)
		}
		stageSet[id] = true
	}
	ruleSet := map[string]bool{}
	for _, id := range rules {
		stage, ok := stageOwningRule(id)
		if !ok {
			return selection, output.NewError(
				output.CodeUsage,
				fmt.Sprintf("unknown verify rule %q", id),
				output.ExitUsage,
				"Rule IDs are listed in andurel verify --help and in the rules field of andurel commands --json.",
			)
		}
		ruleSet[id] = true
		stageSet[stage.ID] = true
	}
	selectAll := len(only) == 0 && len(rules) == 0
	for _, stage := range verifyStages() {
		if selectAll || stageSet[stage.ID] {
			selection.StageIDs = append(selection.StageIDs, stage.ID)
		}
	}
	if len(selection.StageIDs) == 0 {
		return selection, output.NewError(
			output.CodeUsage,
			"verify selection matched no stages",
			output.ExitUsage,
			"Use one of: "+strings.Join(verifyStageIDs(), ", ")+".",
		)
	}
	for _, rule := range layers.Catalog() {
		stage, ok := stageOwningRule(rule.ID)
		if !ok || !stageSelected(selection.StageIDs, stage.ID) {
			continue
		}
		if selection.ruleFiltered && !ruleSet[rule.ID] {
			continue
		}
		selection.RuleIDs = append(selection.RuleIDs, rule.ID)
	}
	return selection, nil
}

func splitVerifyNames(values []string) []string {
	names := make([]string, 0, len(values))
	for _, value := range values {
		for name := range strings.SplitSeq(value, ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}

func stageSelected(stageIDs []string, id string) bool {
	return slices.Contains(stageIDs, id)
}

func stageOwningRule(ruleID string) (verifyStageSpec, bool) {
	for _, stage := range verifyStages() {
		if slices.Contains(stage.RuleIDs, ruleID) {
			return stage, true
		}
	}
	return verifyStageSpec{}, false
}

// with returns the selection with a stage added in canonical order.
func (s verifySelection) with(stageID string) verifySelection {
	if stageSelected(s.StageIDs, stageID) {
		return s
	}
	stages := make([]string, 0, len(s.StageIDs)+1)
	added := false
	for _, stage := range verifyStages() {
		switch {
		case stage.ID == stageID:
			stages = append(stages, stage.ID)
			added = true
		case stageSelected(s.StageIDs, stage.ID):
			stages = append(stages, stage.ID)
		}
	}
	if !added {
		return s
	}
	s.StageIDs = stages
	return s
}

// without returns the selection with a stage removed. Its rules stay in
// RuleIDs so a skipped stage's rule view remains explicit.
func (s verifySelection) without(stageID string) verifySelection {
	stages := make([]string, 0, len(s.StageIDs))
	for _, id := range s.StageIDs {
		if id != stageID {
			stages = append(stages, id)
		}
	}
	s.StageIDs = stages
	return s
}

// stageRules reports the selected/executed/skipped rule view for one stage.
func (s verifySelection) stageRules(stage verifyStageSpec, executed []string) *verifyStepRules {
	if len(stage.RuleIDs) == 0 {
		return nil
	}
	selected := make([]string, 0, len(stage.RuleIDs))
	for _, rule := range stage.RuleIDs {
		if !s.ruleFiltered || slices.Contains(s.RuleIDs, rule) {
			selected = append(selected, rule)
		}
	}
	if len(selected) == 0 {
		return nil
	}
	rules := &verifyStepRules{}
	for _, rule := range selected {
		rules.Selected = append(rules.Selected, rule)
		if slices.Contains(executed, rule) {
			rules.Executed = append(rules.Executed, rule)
		} else {
			rules.Skipped = append(rules.Skipped, rule)
		}
	}
	return rules
}

// verifyScopeWidening explains why a step checked more than the changed files.
type verifyScopeWidening struct {
	Step   string `json:"step"`
	Reason string `json:"reason"`
}

// verifyScope is the applied scope recorded in the JSON report.
type verifyScope struct {
	Mode         string                `json:"mode"`
	Base         string                `json:"base,omitempty"`
	ChangedFiles []string              `json:"changed_files,omitempty"`
	GoPackages   []string              `json:"go_packages,omitempty"`
	Stages       []string              `json:"stages"`
	Rules        []string              `json:"rules,omitempty"`
	Widened      []verifyScopeWidening `json:"widened,omitempty"`
	Noop         bool                  `json:"noop,omitempty"`
}

const (
	verifyScopeAll     = "all"
	verifyScopeChanged = "changed"
	verifyDefaultBase  = "HEAD"
)

// verifyChangedEntry is one changed path with its git status letter. A rename
// or copy contributes both its old and new path.
type verifyChangedEntry struct {
	Status string
	Path   string
}

// verifyPlan is the resolved execution plan for one verify run.
type verifyPlan struct {
	scope    verifyScope
	changed  bool
	noop     bool
	goFiles  []string // changed and still-present Go files, for gofmt
	goWhole  bool     // a global change widened Go checks to the whole module
	patterns []string // Go package patterns for fix/vet/layers/tests
	// run reports whether a selected stage has work in changed mode. In
	// whole-project mode every selected stage runs.
	run         map[string]bool
	skipReasons map[string]string
}

// buildVerifyPlan resolves positional patterns, changed files, Go packages,
// and per-stage applicability. Git problems are usage errors; a broken Go
// module widens to ./... so the Go stages report the real failure.
func buildVerifyPlan(
	rootDir string,
	selection verifySelection,
	changed bool,
	base string,
	patterns []string,
) (verifyPlan, error) {
	plan := verifyPlan{
		run:         map[string]bool{},
		skipReasons: map[string]string{},
		scope: verifyScope{
			Stages: append([]string(nil), selection.StageIDs...),
			Rules:  append([]string(nil), selection.RuleIDs...),
		},
	}
	for _, id := range selection.StageIDs {
		plan.run[id] = true
	}
	if !changed {
		plan.scope.Mode = verifyScopeAll
		plan.patterns = goPatternArgs(patterns)
		return plan, nil
	}
	if len(patterns) > 0 {
		return plan, output.NewError(
			output.CodeUsage,
			"verify --changed cannot be combined with Go package patterns",
			output.ExitUsage,
			"Run either 'andurel verify --changed' for the changed scope or 'andurel verify ./pkg/...' for an explicit package scope.",
		)
	}
	if strings.TrimSpace(base) == "" {
		base = verifyDefaultBase
	}
	entries, err := collectChangedFiles(rootDir, base)
	if err != nil {
		return plan, err
	}
	plan.changed = true
	plan.scope.Mode = verifyScopeChanged
	plan.scope.Base = base
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		files = append(files, entry.Path)
	}
	plan.scope.ChangedFiles = files
	if len(entries) == 0 {
		plan.noop = true
		plan.scope.Noop = true
		for _, id := range selection.StageIDs {
			plan.run[id] = false
			plan.skipReasons[id] = "no changed files in scope"
		}
		return plan, nil
	}

	goFiles, goPackages, goWhole, goWidenReason := resolveChangedGoScope(rootDir, entries)
	plan.goFiles = goFiles
	plan.goWhole = goWhole
	if goWhole {
		plan.patterns = []string{"./..."}
		plan.scope.Widened = append(plan.scope.Widened, verifyScopeWidening{
			Step:   "Go checks",
			Reason: goWidenReason,
		})
	} else if len(goPackages) > 0 {
		plan.patterns = append([]string(nil), goPackages...)
	}
	plan.scope.GoPackages = goPackages

	skipStage := func(id, reason string) {
		plan.run[id] = false
		plan.skipReasons[id] = reason
	}
	for _, id := range []string{verifyStageGofmt, verifyStageGoFix, verifyStageGoVet, verifyStageLayers} {
		if !stageSelected(selection.StageIDs, id) {
			continue
		}
		if goWhole {
			if id == verifyStageGofmt {
				plan.scope.Widened = append(plan.scope.Widened, verifyScopeWidening{
					Step:   "gofmt",
					Reason: "a global Go or project file changed, so gofmt checks the whole module",
				})
			}
			continue
		}
		if len(goPackages) == 0 {
			skipStage(id, "no changed Go files in scope")
		}
	}
	if stageSelected(selection.StageIDs, verifyStageGoTest) {
		if goWhole || len(goPackages) > 0 {
			plan.scope.Widened = append(plan.scope.Widened, verifyScopeWidening{
				Step:   "go test",
				Reason: "changed packages and their reverse dependencies run tests; unchanged packages do not",
			})
		} else {
			skipStage(verifyStageGoTest, "no changed Go files in scope")
		}
	}
	if stageSelected(selection.StageIDs, verifyStageFrontend) {
		if changedFilesAffectFrontend(entries) {
			plan.scope.Widened = append(plan.scope.Widened, verifyScopeWidening{
				Step:   "frontend",
				Reason: "the package.json typecheck and route-usage analysis always check the whole frontend project",
			})
		} else {
			skipStage(verifyStageFrontend, "no changed frontend files in scope")
		}
	}
	if stageSelected(selection.StageIDs, verifyStageGenerated) {
		if changedFilesAffectGenerated(entries) {
			plan.scope.Widened = append(plan.scope.Widened, verifyScopeWidening{
				Step:   "generated",
				Reason: "generated output is whole-project: regeneration reads route, handler, template, query, and email inputs across the project",
			})
		} else {
			skipStage(verifyStageGenerated, "no changed generation inputs in scope")
		}
	}
	if stageSelected(selection.StageIDs, verifyStageContent) {
		if changedFilesAffectContent(rootDir, entries) {
			plan.scope.Widened = append(plan.scope.Widened, verifyScopeWidening{
				Step:   "content",
				Reason: "declared content invariants cover the whole configured content tree and its declared catalog/checker inputs",
			})
		} else {
			skipStage(verifyStageContent, "no changed content files in scope")
		}
	}
	if len(plan.scope.Widened) == 0 {
		plan.scope.Widened = nil
	}
	return plan, nil
}

// collectChangedFiles returns tracked changes since base plus untracked files.
// Deletions and renames are included; ignored files follow git's ignore rules.
func collectChangedFiles(rootDir, base string) ([]verifyChangedEntry, error) {
	if _, err := runVerifyGit(rootDir, "rev-parse", "--is-inside-work-tree"); err != nil {
		return nil, output.NewError(
			output.CodeUsage,
			"verify --changed requires a Git repository",
			output.ExitUsage,
			"Run it inside the project's Git work tree, or run 'andurel verify' without --changed.",
		)
	}
	if _, err := runVerifyGit(rootDir, "rev-parse", "--verify", "--quiet", base+"^{commit}"); err != nil {
		return nil, output.NewError(
			output.CodeUsage,
			fmt.Sprintf("verify --changed base %q is not a valid Git revision", base),
			output.ExitUsage,
			"Pass an existing revision, for example --changed --base HEAD~1, or omit --base to compare with HEAD.",
		)
	}
	diffOut, err := runVerifyGit(rootDir, "diff", "--name-status", "-z", "--find-renames", base, "--")
	if err != nil {
		return nil, err
	}
	entries := parseGitNameStatus(diffOut)
	untrackedOut, err := runVerifyGit(rootDir, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	for name := range strings.SplitSeq(untrackedOut, "\x00") {
		if name == "" {
			continue
		}
		entries = append(entries, verifyChangedEntry{Status: "??", Path: name})
	}
	slices.SortFunc(entries, func(a, b verifyChangedEntry) int {
		if order := strings.Compare(a.Path, b.Path); order != 0 {
			return order
		}
		return strings.Compare(a.Status, b.Status)
	})
	return slices.CompactFunc(entries, func(a, b verifyChangedEntry) bool {
		return a.Path == b.Path && a.Status == b.Status
	}), nil
}

// parseGitNameStatus parses `git diff --name-status -z` records. Rename and
// copy records carry an old and a new path; both are returned.
func parseGitNameStatus(data string) []verifyChangedEntry {
	fields := strings.Split(data, "\x00")
	entries := make([]verifyChangedEntry, 0, len(fields))
	for index := 0; index < len(fields); {
		status := strings.TrimSpace(fields[index])
		index++
		if status == "" {
			continue
		}
		if index >= len(fields) {
			break
		}
		first := fields[index]
		index++
		if status[0] == 'R' || status[0] == 'C' {
			if index >= len(fields) {
				break
			}
			second := fields[index]
			index++
			entries = append(entries, verifyChangedEntry{Status: status, Path: first})
			entries = append(entries, verifyChangedEntry{Status: status, Path: second})
			continue
		}
		entries = append(entries, verifyChangedEntry{Status: status, Path: first})
	}
	return entries
}

func runVerifyGit(rootDir string, args ...string) (string, error) {
	fullArgs := append([]string{"-C", rootDir}, args...)
	cmd := exec.Command("git", fullArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), detail)
	}
	return stdout.String(), nil
}

// resolveChangedGoScope maps changed Go files to package import paths and adds
// their in-module reverse dependencies, because an exported change can break a
// dependent package that the changed package alone does not type-check.
func resolveChangedGoScope(
	rootDir string,
	entries []verifyChangedEntry,
) (goFiles, goPackages []string, whole bool, widenReason string) {
	global := false
	changedDirs := map[string]bool{}
	for _, entry := range entries {
		if isGlobalGoChange(entry.Path) {
			global = true
		}
		if !strings.HasSuffix(entry.Path, ".go") {
			continue
		}
		changedDirs[path.Dir(entry.Path)] = true
		if entry.Status == "D" {
			continue
		}
		if _, err := os.Stat(filepath.Join(rootDir, filepath.FromSlash(entry.Path))); err != nil {
			continue
		}
		goFiles = append(goFiles, entry.Path)
	}
	slices.Sort(goFiles)
	goFiles = slices.Compact(goFiles)
	if global {
		return goFiles, nil, true,
			"go.mod, go.sum, go.work, go.work.sum, andurel.toml, andurel.lock, or bin/ changed, so Go checks cover the whole module"
	}
	if len(changedDirs) == 0 {
		return nil, nil, false, ""
	}
	packages, err := listGoPackages(rootDir)
	if err != nil {
		return goFiles, nil, true,
			"go list failed (" + err.Error() + "), so Go checks cover the whole module"
	}
	importByDir := map[string]string{}
	dependents := map[string][]string{}
	for _, pkg := range packages {
		rel, err := filepath.Rel(rootDir, pkg.Dir)
		if err != nil {
			continue
		}
		dir := filepath.ToSlash(rel)
		if dir == "." {
			dir = ""
		}
		importByDir[dir] = pkg.ImportPath
		for _, imported := range append(append(append([]string{}, pkg.Imports...), pkg.TestImports...), pkg.XTestImports...) {
			dependents[imported] = append(dependents[imported], pkg.ImportPath)
		}
	}
	queue := make([]string, 0)
	for dir := range changedDirs {
		if dir == "." {
			dir = ""
		}
		if importPath, ok := importByDir[dir]; ok {
			queue = append(queue, importPath)
		}
	}
	affected := map[string]bool{}
	for len(queue) > 0 {
		importPath := queue[0]
		queue = queue[1:]
		if affected[importPath] {
			continue
		}
		affected[importPath] = true
		queue = append(queue, dependents[importPath]...)
	}
	goPackages = make([]string, 0, len(affected))
	for _, pkg := range packages {
		if affected[pkg.ImportPath] {
			goPackages = append(goPackages, pkg.ImportPath)
		}
	}
	slices.Sort(goPackages)
	return goFiles, goPackages, false, ""
}

func isGlobalGoChange(file string) bool {
	switch file {
	case "go.mod", "go.sum", "go.work", "go.work.sum", "andurel.toml", "andurel.lock":
		return true
	}
	return strings.HasPrefix(file, "bin/")
}

type goListPackage struct {
	Dir          string
	ImportPath   string
	Imports      []string
	TestImports  []string
	XTestImports []string
}

// listGoPackages loads the module's package graph for reverse-dependency
// scoping. Load errors (-e) keep partial results; a broken module is widened by
// the caller.
func listGoPackages(rootDir string) ([]goListPackage, error) {
	format := "{{.Dir}}\t{{.ImportPath}}\t{{join .Imports \",\"}}\t{{join .TestImports \",\"}}\t{{join .XTestImports \",\"}}"
	out, err := runVerifyCommand(rootDir, "go", "list", "-e", "-f", format, "./...")
	if err != nil {
		return nil, fmt.Errorf("%s", strings.TrimSpace(out))
	}
	packages := make([]goListPackage, 0)
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		pkg := goListPackage{Dir: fields[0], ImportPath: fields[1]}
		if len(fields) > 2 && fields[2] != "" {
			pkg.Imports = strings.Split(fields[2], ",")
		}
		if len(fields) > 3 && fields[3] != "" {
			pkg.TestImports = strings.Split(fields[3], ",")
		}
		if len(fields) > 4 && fields[4] != "" {
			pkg.XTestImports = strings.Split(fields[4], ",")
		}
		packages = append(packages, pkg)
	}
	if len(packages) == 0 {
		return nil, fmt.Errorf("no Go packages matched the ./... pattern")
	}
	return packages, nil
}

func changedFilesAffectFrontend(entries []verifyChangedEntry) bool {
	for _, entry := range entries {
		switch strings.ToLower(path.Ext(entry.Path)) {
		case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".css", ".vue", ".svelte":
			return true
		}
		switch entry.Path {
		case "package.json", "tsconfig.json":
			return true
		}
		if strings.HasPrefix(entry.Path, "resources/") {
			return true
		}
	}
	return false
}

func changedFilesAffectGenerated(entries []verifyChangedEntry) bool {
	for _, entry := range entries {
		switch {
		case strings.HasSuffix(entry.Path, ".md"),
			strings.HasSuffix(entry.Path, ".mdx"),
			strings.HasPrefix(entry.Path, "content/"),
			strings.HasPrefix(entry.Path, "docs/"),
			strings.HasPrefix(entry.Path, "plans/"),
			entry.Path == contentManifestName,
			entry.Path == "LICENSE",
			entry.Path == "SECURITY.md":
			continue
		}
		return true
	}
	return false
}

func changedFilesAffectContent(rootDir string, entries []verifyChangedEntry) bool {
	for _, entry := range entries {
		if entry.Path == contentManifestName {
			return true
		}
		switch strings.ToLower(path.Ext(entry.Path)) {
		case ".md", ".mdx":
			return true
		}
		// Deleting or renaming an asset can break a local link even when the
		// changed path itself is not Markdown.
		if strings.HasPrefix(entry.Status, "D") || strings.HasPrefix(entry.Status, "R") {
			return true
		}
	}
	cfg, diagnostics := content.Load(rootDir)
	if cfg == nil {
		// A present but unloadable declaration has unknown inputs; a missing
		// declaration has nothing to check.
		return len(diagnostics) > 0
	}
	declared := contentDeclaredInputs(cfg)
	for _, entry := range entries {
		if contentInputMatches(entry.Path, declared) {
			return true
		}
	}
	// A declared checker is opaque: its argv and imports can read any project
	// file, so a change whose impact cannot be determined conservatively runs
	// the content stage. This is what catches a changed Go catalog checker or
	// catalog source that is not itself Markdown.
	return len(cfg.Checkers) > 0
}

// contentDeclaredInputs lists the project paths a content declaration names
// directly. Checker argv is intentionally absent: a checker is treated as
// opaque, so any changed file runs the stage.
func contentDeclaredInputs(cfg *content.Config) []string {
	inputs := make([]string, 0, 2+len(cfg.Required)+len(cfg.Catalog))
	if cfg.CatalogFile != "" {
		inputs = append(inputs, cfg.CatalogFile)
	}
	inputs = append(inputs, cfg.Required...)
	for _, entry := range cfg.Catalog {
		if entry.File != "" {
			inputs = append(inputs, entry.File)
		}
	}
	for _, checker := range cfg.Checkers {
		if checker.Dir != "" {
			inputs = append(inputs, checker.Dir)
		}
	}
	return inputs
}

func contentInputMatches(changed string, declared []string) bool {
	changed = path.Clean(strings.TrimSpace(changed))
	for _, input := range declared {
		input = path.Clean(strings.TrimSpace(filepath.ToSlash(input)))
		if input == "" || input == "." {
			continue
		}
		if changed == input || strings.HasPrefix(changed, input+"/") {
			return true
		}
	}
	return false
}
