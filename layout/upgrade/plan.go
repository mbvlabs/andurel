package upgrade

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/mbvlabs/andurel/layout"
	"github.com/mbvlabs/andurel/layout/templates"
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
	includeInertiaMigration := layout.IsSupportedInertiaAdapter(lock.ScaffoldConfig.Inertia)
	if crossesVersion(plan.fromVersion, plan.toVersion, sessionCookieRecoveryVersion) ||
		(includeInertiaMigration && crossesVersion(plan.fromVersion, plan.toVersion, inertiaRendererInjectionVersion)) {
		modulePath, err := resolveModulePath(u.projectRoot)
		if err != nil {
			return nil, fmt.Errorf("resolve module path for manual actions: %w", err)
		}
		plan.manualActions, err = manualActionsForUpgrade(
			plan.fromVersion,
			plan.toVersion,
			modulePath,
			includeInertiaMigration,
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
	if err := u.addInertiaV3ClientMigration(plan, lock); err != nil {
		return nil, err
	}
	if err := u.addNativeInertiaMigration(plan, lock); err != nil {
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

func (u *Upgrader) addInertiaV3ClientMigration(plan *upgradePlan, lock *layout.AndurelLock) error {
	if !layout.IsSupportedInertiaAdapter(lock.ScaffoldConfig.Inertia) {
		return nil
	}

	packagePath := filepath.Join(u.projectRoot, "package.json")
	packageJSON, err := os.ReadFile(packagePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read package.json for Inertia v3 migration: %w", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(packageJSON, &manifest); err != nil {
		return fmt.Errorf("parse package.json for Inertia v3 migration: %w", err)
	}
	dependencies := jsonObject(manifest, "dependencies")
	devDependencies := jsonObject(manifest, "devDependencies")
	adapterPackage := map[string]string{
		"vue":    "@inertiajs/vue3",
		"react":  "@inertiajs/react",
		"svelte": "@inertiajs/svelte",
	}[lock.ScaffoldConfig.Inertia]
	currentAdapter, _ := dependencies[adapterPackage].(string)
	if currentAdapter == "" {
		return fmt.Errorf("package.json is missing Inertia adapter %s", adapterPackage)
	}

	dependencies[adapterPackage] = "^3.6.1"
	devDependencies["@inertiajs/vite"] = "^3.6.1"
	devDependencies["@types/node"] = "^22.0.0"
	devDependencies["vite"] = "^7.0.0"
	switch lock.ScaffoldConfig.Inertia {
	case "vue":
		devDependencies["@vitejs/plugin-vue"] = "^6.0.0"
	case "react":
		devDependencies["@vitejs/plugin-react"] = "^5.0.0"
	case "svelte":
		devDependencies["@sveltejs/vite-plugin-svelte"] = "^6.0.0"
	}
	manifest["type"] = "module"
	scripts := jsonObject(manifest, "scripts")
	if build, _ := scripts["build"].(string); build == "vite build" {
		entry := "resources/js/ssr.ts"
		if lock.ScaffoldConfig.Inertia == "react" {
			entry = "resources/js/ssr.tsx"
		}
		scripts["build"] = "vite build && vite build --ssr " + entry + " --outDir assets/dist/ssr"
	}
	updatedPackageJSON, err := marshalJSONNoEscape(manifest)
	if err != nil {
		return fmt.Errorf("encode migrated package.json: %w", err)
	}
	if err := plan.addReplacement(u.projectRoot, "package.json", updatedPackageJSON, false); err != nil {
		return err
	}

	const vitePath = "vite.config.ts"
	viteConfig, err := os.ReadFile(filepath.Join(u.projectRoot, vitePath))
	if err != nil {
		return fmt.Errorf("read Vite config for Inertia v3 migration: %w", err)
	}
	if !bytes.Contains(viteConfig, []byte("@inertiajs/vite")) {
		viteConfig = bytes.Replace(viteConfig,
			[]byte("import { defineConfig } from 'vite'\n"),
			[]byte("import { defineConfig } from 'vite'\nimport inertia from '@inertiajs/vite'\n"),
			1,
		)
		viteConfig = bytes.Replace(viteConfig, []byte("plugins: ["), []byte("plugins: [inertia({ ssr: false }), "), 1)
		if err := plan.addReplacement(u.projectRoot, vitePath, viteConfig, false); err != nil {
			return err
		}
	}

	ssrTemplate, ssrPath := "inertia_assets_ssr.tmpl", "resources/js/ssr.ts"
	if lock.ScaffoldConfig.Inertia == "react" {
		ssrTemplate, ssrPath = "inertia_react_assets_ssr.tmpl", "resources/js/ssr.tsx"
	} else if lock.ScaffoldConfig.Inertia == "svelte" {
		ssrTemplate = "inertia_svelte_assets_ssr.tmpl"
	}
	if _, err := os.Stat(filepath.Join(u.projectRoot, ssrPath)); os.IsNotExist(err) {
		entry, readErr := fs.ReadFile(templates.Files, ssrTemplate)
		if readErr != nil {
			return fmt.Errorf("read Inertia v3 SSR entry template: %w", readErr)
		}
		if err := plan.addReplacement(u.projectRoot, ssrPath, entry, false); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("inspect Inertia SSR entry: %w", err)
	}

	modulePath, err := resolveModulePath(u.projectRoot)
	if err != nil {
		return fmt.Errorf("resolve module path for Inertia router migration: %w", err)
	}
	routerPath := "router/router.go"
	routerFile, err := os.ReadFile(filepath.Join(u.projectRoot, routerPath))
	if err != nil {
		return fmt.Errorf("read Inertia router for v3 migration: %w", err)
	}
	updatedRouter := bytes.Replace(routerFile,
		[]byte("\t\tmiddleware.RegisterRequestMeta,\n\t\trenderer.Middleware(),\n"),
		[]byte("\t\trenderer.Middleware(),\n\t\tmiddleware.RegisterRequestMeta,\n"),
		1,
	)
	routerMigrationNote := ""
	if !bytes.Contains(updatedRouter, []byte("SetReflashHandler")) {
		marker := []byte("\t// Order matters:")
		if bytes.Contains(updatedRouter, marker) {
			inertiaImport := []byte("\t\"" + modulePath + "/internal/inertia\"\n")
			requestImport := []byte("\t\"" + modulePath + "/internal/request\"\n")
			if !bytes.Contains(updatedRouter, requestImport) {
				updatedRouter = bytes.Replace(updatedRouter, inertiaImport, append(inertiaImport, requestImport...), 1)
			}
			setup := []byte("\tif err := renderer.SetReflashHandler(func(etx *echo.Context) error {\n" +
				"\t\tflashes := request.ExtractContext[[]cookies.FlashMessage](etx.Request().Context(), request.SessionFlashesKey)\n" +
				"\t\treturn cookies.Reflash(etx, flashes)\n" +
				"\t}); err != nil {\n\t\treturn nil, err\n\t}\n\n")
			updatedRouter = bytes.Replace(updatedRouter, marker, append(setup, marker...), 1)
		} else {
			routerMigrationNote = " The customized router could not be wired automatically; configure Renderer.SetReflashHandler with cookies.Reflash before serving requests so redirect chains preserve flash data."
		}
	}
	if err := plan.addReplacement(u.projectRoot, routerPath, updatedRouter, false); err != nil {
		return err
	}

	flashPath := "router/cookies/flash.go"
	flashFile, err := os.ReadFile(filepath.Join(u.projectRoot, flashPath))
	if err != nil {
		return fmt.Errorf("read flash helpers for Inertia v3 migration: %w", err)
	}
	if !bytes.Contains(flashFile, []byte("func Reflash(")) {
		flashFile = append(flashFile, []byte("\n// Reflash restores flashes consumed by a request that returns a redirect.\n"+
			"func Reflash(c *echo.Context, flashes []FlashMessage) error {\n"+
			"\tif len(flashes) == 0 { return nil }\n"+
			"\tsess, err := getSession(flashSession, c)\n\tif err != nil { return err }\n"+
			"\tfor _, flash := range flashes { sess.AddFlash(flash, flashSessionName) }\n"+
			"\treturn sess.Save(c.Request(), c.Response())\n}\n")...)
		if err := plan.addReplacement(u.projectRoot, flashPath, flashFile, false); err != nil {
			return err
		}
	}

	plan.manualActions = append(plan.manualActions, ManualAction{
		ID:    "inertia-v3-client-entry",
		Title: "Review the application-owned Inertia v3 client entry",
		Instructions: "Andurel upgraded package.json and vite.config.ts and added the SSR entry without overwriting resources/js/app.*. " +
			"Update that application-owned client entry to hydrate when #app has data-server-rendered=\"true\" and read flash data from page.flash instead of page.props.flash. " +
			"Compare it with a newly generated application for the selected frontend before enabling INERTIA_SSR_ENABLED." + routerMigrationNote,
	})
	return nil
}

func jsonObject(parent map[string]any, key string) map[string]any {
	if object, ok := parent[key].(map[string]any); ok {
		return object
	}
	object := make(map[string]any)
	parent[key] = object
	return object
}

func (u *Upgrader) addNativeInertiaMigration(plan *upgradePlan, lock *layout.AndurelLock) error {
	if !layout.IsSupportedInertiaAdapter(lock.ScaffoldConfig.Inertia) {
		return nil
	}

	const goModPath = "go.mod"
	goMod, err := os.ReadFile(filepath.Join(u.projectRoot, goModPath))
	if err != nil {
		return fmt.Errorf("read go.mod for native Inertia migration: %w", err)
	}
	gonertiaRequirement := regexp.MustCompile(`(?m)^\s*github\.com/romsar/gonertia/v3\s+\S+\s*$`)
	if !gonertiaRequirement.Match(goMod) {
		return nil
	}

	andurelRequirementPattern := regexp.MustCompile(`(?m)^\s*github\.com/mbvlabs/andurel\s+\S+\s*$`)
	updatedGoMod := gonertiaRequirement.ReplaceAll(goMod, nil)
	updatedGoMod = andurelRequirementPattern.ReplaceAll(updatedGoMod, nil)
	if err := plan.addReplacement(u.projectRoot, goModPath, updatedGoMod, false); err != nil {
		return fmt.Errorf("replace Gonertia dependency: %w", err)
	}

	const rootPath = "internal/inertia/root.templ"
	if _, err := os.Stat(filepath.Join(u.projectRoot, rootPath)); os.IsNotExist(err) {
		root, readErr := fs.ReadFile(templates.Files, "inertia_framework_root_templ.tmpl")
		if readErr != nil {
			return fmt.Errorf("read native Inertia root template: %w", readErr)
		}
		if err := plan.addReplacement(u.projectRoot, rootPath, root, false); err != nil {
			return fmt.Errorf("add native Inertia root: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("inspect native Inertia root: %w", err)
	}

	const mainPath = "cmd/app/main.go"
	mainFile, err := os.ReadFile(filepath.Join(u.projectRoot, mainPath))
	if err != nil {
		return fmt.Errorf("read Inertia application entrypoint: %w", err)
	}
	updatedMain := bytes.Replace(mainFile,
		[]byte("\t\tassets.Files,\n\t\t\"inertia/root.go.html\",\n"),
		[]byte("\t\tassets.Files,\n"),
		1,
	)
	updatedMain = bytes.Replace(updatedMain,
		[]byte("inertia.WithRequestProps(func(ctx context.Context) inertia.Props"),
		[]byte("inertia.WithRequestFlash(func(ctx context.Context) any"),
		1,
	)
	updatedMain = bytes.Replace(updatedMain,
		[]byte(`return inertia.Props{"flash": flashes}`),
		[]byte("return flashes"),
		1,
	)
	if !bytes.Equal(mainFile, updatedMain) {
		if err := plan.addReplacement(u.projectRoot, mainPath, updatedMain, false); err != nil {
			return fmt.Errorf("update native Inertia provider: %w", err)
		}
	}

	plan.manualActions = append(plan.manualActions, ManualAction{
		ID:    "andurel-owned-inertia-v3",
		Title: "Finish the native Inertia v3 migration",
		Instructions: "Andurel removed the generated Gonertia facade and dependency and generated the self-contained internal/inertia protocol package plus internal/inertia/root.templ. " +
			"Run `andurel template generate`, `go mod tidy`, `gofmt`, `go fix ./...`, and `go vet ./...`. " +
			"Compare cmd/app/main.go and config/app.go with a newly generated application, add the INERTIA_SSR_* environment keys, and wire NewSSRRuntime plus its Fx lifecycle before enabling INERTIA_SSR_ENABLED; application-owned startup and configuration code is not overwritten automatically. " +
			"If application-owned code uses WithGonertiaOptions or imports github.com/romsar/gonertia/v3 directly, replace those options with the generated internal/inertia equivalents before verification. " +
			"Review the retired assets/inertia/root.go.html and carry any application-specific metadata into internal/inertia/root.templ before removing the old file.",
	})

	return nil
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
	if _, err := os.Stat(filepath.Join(u.projectRoot, "internal/inertia/root.templ")); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect native Inertia root: %w", err)
	}

	const embeddedPath = "assets/inertia/root.go.html"
	if _, err := os.Stat(filepath.Join(u.projectRoot, embeddedPath)); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect embedded Inertia root: %w", err)
	}

	const legacyPath = "views/root.go.html"
	if _, err := os.Stat(filepath.Join(u.projectRoot, legacyPath)); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect existing Inertia root %s: %w", legacyPath, err)
	}
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

// marshalJSONNoEscape encodes value as indented JSON without HTML escaping,
// preserving characters such as && in script values.
func marshalJSONNoEscape(value any) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
