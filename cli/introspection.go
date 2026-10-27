package cli

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mbvlabs/andurel/v2/cli/internal/layers"
	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/mbvlabs/andurel/v2/layout"
	"github.com/spf13/cobra"
)

type projectInfo struct {
	Root           string                 `json:"root"`
	Module         string                 `json:"module,omitempty"`
	GoVersion      string                 `json:"go_version,omitempty"`
	AndurelVersion string                 `json:"andurel_version,omitempty"`
	ScaffoldConfig *layout.ScaffoldConfig `json:"scaffold_config,omitempty"`
	DatabaseConfig *layout.DatabaseConfig `json:"database_config,omitempty"`
	Tools          []toolInfo             `json:"tools"`
}

type projectItem struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Kind string `json:"kind,omitempty"`
	// Entities holds the type→column mapping for domain files. It is only
	// populated by andurel inspect domains; every other list leaves it empty.
	Entities []domainEntity `json:"entities,omitempty"`
	// Handlers holds the handler types declared in a handlers file. It is
	// only populated by andurel inspect handlers.
	Handlers []handlerInfo `json:"handlers,omitempty"`
}

type toolInfo struct {
	Name       string `json:"name"`
	Version    string `json:"version,omitempty"`
	Source     string `json:"source,omitempty"`
	Path       string `json:"path,omitempty"`
	BinaryPath string `json:"binary_path,omitempty"`
	Installed  bool   `json:"installed"`
}

func newProjectInfoCommand() *cobra.Command {
	projectCmd := &cobra.Command{
		Use:   "project",
		Short: "Inspect Andurel project metadata",
		Long: `Inspect project metadata from go.mod, andurel.toml, and andurel.lock.

andurel.toml is the human-edited manifest: framework version, project name
and Inertia adapter, database engine/null type, and the pinned version of each
tool. andurel.lock is the CLI-owned digest lock: per-platform SHA-256 hashes
of the tool downloads.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectInfo(cmd)
		},
	}
	setAgentMetadata(
		projectCmd,
		"introspection",
		"Read-only project metadata. Prefer this before generation.",
	)

	infoCmd := &cobra.Command{
		Use:   "info",
		Short: "Show project metadata",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectInfo(cmd)
		},
	}
	setAgentMetadata(
		infoCmd,
		"introspection",
		"Read-only project metadata. Prefer this before generation.",
	)
	projectCmd.AddCommand(infoCmd)
	return projectCmd
}

func runProjectInfo(cmd *cobra.Command) error {
	rootDir, err := findGoModRoot()
	if err != nil {
		return err
	}
	info, err := collectProjectInfo(rootDir)
	if err != nil {
		return err
	}
	return output.OK(cmd, info, "Project info loaded")
}

func collectProjectInfo(rootDir string) (projectInfo, error) {
	module, goVersion, _ := readGoModMetadata(rootDir)
	lock, err := layout.ReadLockFile(rootDir)
	if err != nil {
		return projectInfo{}, err
	}
	info := projectInfo{
		Root:           rootDir,
		Module:         module,
		GoVersion:      goVersion,
		AndurelVersion: lock.Version,
		ScaffoldConfig: lock.ScaffoldConfig,
		DatabaseConfig: lock.DatabaseConfig,
		Tools:          toolInfos(rootDir, lock),
	}
	return info, nil
}

func newRoutesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "routes",
		Short: "List route manifest",
		Long: `List route metadata extracted from routes/*.go.

The manifest reports actual URL paths, route names, parameter names and
types, and the Go source location for each route variable. In this command's
output, path means the route URL path. The declaring Go file is reported as
source_file in structured output.`,
		Example: `  andurel inspect routes
  andurel inspect routes --json
  andurel inspect routes --jq .routes`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}
			manifest, err := collectRouteManifest(rootDir)
			if err != nil {
				return err
			}
			opts, err := output.ParseOptions(cmd)
			if err != nil {
				return err
			}
			if opts.Mode == output.ModeHuman {
				if opts.Quiet {
					return nil
				}
				return renderRouteManifestHuman(cmd.OutOrStdout(), manifest)
			}

			summary := fmt.Sprintf("Listed %d routes", len(manifest.Routes))
			if len(manifest.Skipped) > 0 {
				summary = fmt.Sprintf("%s (%d skipped)", summary, len(manifest.Skipped))
			}
			return output.OK(cmd, manifest, summary)
		},
	}
	setAgentMetadata(
		cmd,
		"introspection",
		"Read-only route manifest with actual URL paths, route names, params, and source files.",
	)
	return cmd
}

func newMigrationsCommand() *cobra.Command {
	return readOnlyListCommand(
		"migrations",
		"List migration files",
		"migrations",
		".sql",
	)
}

func newDomainsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "domains",
		Short: "List domain files with their type to column mapping",
		Long: `List files under domains/ and report each entity's type to column
mapping: the structs, their andurel-tagged fields, the table, whether the
entity has a Validate method, and whether its receiver (e.g. Users for User)
is wired: constructed with domains.NewUsers(db) in internal/runtime.

An entity with wired=false exists but nothing constructs its receiver:
build it in internal/runtime with domains.New<Receiver>(db) and pass it to
the services and handlers that use it.

When narsilc-generated queries are present, each entity also reports
columns_ok, mismatches, and unpopulated_tags. columns_ok is true when every
column an automatic query mapping returns has an exported andurel tag, and
false for a missing column or a duplicate exported tag. It is null when no
generated query instantiates that entity, or when *T implements
narsilc.Transformer[Row] and user code performs the mapping; mismatches and
unpopulated_tags are empty in that unchecked case. Mismatch kinds are
missing-column and duplicate-tag (hard failures) plus type-difference (a note
only: pgx may convert between the Row and entity types, so it does not make
columns_ok false). unpopulated_tags lists tagged fields no automatic query
populated; narsilc leaves extra tags at their zero value.`,
		Example: `  andurel inspect domains --json`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}
			items, err := listProjectFiles(rootDir, "domains", ".go", "domain")
			if err != nil {
				return err
			}
			items = excludeDomainInternalFiles(items)
			module, _, err := readGoModMetadata(rootDir)
			if err != nil {
				return err
			}
			wired := runtimeConstructedReceivers(
				filepath.Join(rootDir, "internal", "runtime"),
				module,
			)
			columnStatuses, err := layers.InspectEntityColumns(rootDir)
			if err != nil {
				return fmt.Errorf("inspect domains: %w", err)
			}
			for i, item := range items {
				if !isDomainEntityFile(item.Path) {
					continue
				}
				items[i].Entities = describeDomainFile(filepath.Join(rootDir, item.Path), wired)
				for j := range items[i].Entities {
					status, ok := columnStatuses[items[i].Entities[j].Name]
					if !ok {
						applyDomainColumnStatus(&items[i].Entities[j], nil)
						continue
					}
					applyDomainColumnStatus(&items[i].Entities[j], &status)
				}
			}
			return output.OK(cmd, items, "List domain files")
		},
	}
	setAgentMetadata(
		cmd,
		"introspection",
		"Read-only domain inventory with the table, type to column mapping, Validate coverage, and runtime wiring. Use --json.",
	)
	return cmd
}

func newHandlersCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "handlers",
		Short: "List handlers with their constructors and routes",
		Long: `List files under handlers/ and report each handler: a type with a
Register(rt *anduin.Router) method. For each handler the output names the
type, its constructor (New<Type> preferred) with the constructor's
parameters and whether it returns an error, and the routes Register mounts
in source order: the routes variable, the route name and URL path from the
route manifest, the HTTP method, and the handler method (action).

A route registered with a string literal reports that literal as its path
and has no variable or name. A file without a handler type is listed with
no handlers.`,
		Example: `  andurel inspect handlers
  andurel inspect handlers --json
  andurel inspect handlers --ids-only`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}
			items, err := listProjectFiles(rootDir, "handlers", ".go", "handler")
			if err != nil {
				return err
			}
			module, _, err := readGoModMetadata(rootDir)
			if err != nil {
				return err
			}
			manifest, err := collectRouteManifest(rootDir)
			if err != nil {
				return err
			}
			items = describeHandlerItems(rootDir, module, items, manifest)

			opts, err := output.ParseOptions(cmd)
			if err != nil {
				return err
			}
			if opts.Mode == output.ModeHuman {
				if opts.Quiet {
					return nil
				}
				return renderHandlersHuman(cmd.OutOrStdout(), items)
			}
			return output.OK(cmd, items, "List handlers")
		},
	}
	setAgentMetadata(
		cmd,
		"introspection",
		"Read-only handler inventory with each handler's constructor, constructor params, and the routes Register mounts (route name, method, path). Use --json.",
	)
	return cmd
}

func newJobsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jobs",
		Short: "List queue job args and workers",
		Long: `List types declared under queue/ and classify each by its Go content,
not by file name or directory:

  job     a type with a Kind() string method (River job args)
  worker  a type that embeds river.WorkerDefaults or defines a Work method

Files such as queue/queue.go that only declare functions are not reported.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}
			items, err := collectQueueItems(rootDir)
			if err != nil {
				return err
			}
			return output.OK(cmd, items, fmt.Sprintf("Listed %d queue types", len(items)))
		},
	}
	setAgentMetadata(cmd, "introspection", "Read-only list of queue job args and workers classified by Go content.")
	return cmd
}

// collectQueueItems walks queue/ and classifies each declared type by its Go
// content: Kind() string marks River job args, river.WorkerDefaults or a Work
// method marks a worker.
func collectQueueItems(rootDir string) ([]projectItem, error) {
	base := filepath.Join(rootDir, "queue")
	if _, err := os.Stat(base); err != nil {
		if os.IsNotExist(err) {
			return []projectItem{}, nil
		}
		return nil, err
	}

	items := []projectItem{}
	err := filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" ||
			strings.HasSuffix(path, "_test.go") {
			return nil
		}
		found, err := classifyQueueFile(rootDir, path)
		if err != nil {
			return err
		}
		items = append(items, found...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Path == items[j].Path {
			return items[i].Name < items[j].Name
		}
		return items[i].Path < items[j].Path
	})
	return items, nil
}

func classifyQueueFile(rootDir, path string) ([]projectItem, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	jobs := map[string]struct{}{}
	workers := map[string]struct{}{}
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.GenDecl:
			if decl.Tok != token.TYPE {
				continue
			}
			for _, spec := range decl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || !embedsRiverWorkerDefaults(typeSpec.Type) {
					continue
				}
				workers[typeSpec.Name.Name] = struct{}{}
			}
		case *ast.FuncDecl:
			if decl.Recv == nil || len(decl.Recv.List) == 0 {
				continue
			}
			recv := namedTypeName(decl.Recv.List[0].Type)
			if recv == "" {
				continue
			}
			switch decl.Name.Name {
			case "Kind":
				if isKindMethod(decl) {
					jobs[recv] = struct{}{}
				}
			case "Work":
				workers[recv] = struct{}{}
			}
		}
	}

	rel, err := filepath.Rel(rootDir, path)
	if err != nil {
		rel = path
	}
	rel = filepath.ToSlash(rel)

	items := []projectItem{}
	add := func(names map[string]struct{}, kind string) {
		found := make([]string, 0, len(names))
		for name := range names {
			found = append(found, name)
		}
		sort.Strings(found)
		for _, name := range found {
			items = append(items, projectItem{
				Name: name,
				Path: rel,
				Kind: kind,
			})
		}
	}
	add(workers, "worker")
	add(jobs, "job")
	return items, nil
}

// embedsRiverWorkerDefaults reports whether a type expression embeds
// river.WorkerDefaults (optionally instantiated with a job args type).
func embedsRiverWorkerDefaults(expr ast.Expr) bool {
	switch typed := expr.(type) {
	case *ast.StructType:
		if typed.Fields == nil {
			return false
		}
		for _, field := range typed.Fields.List {
			if len(field.Names) > 0 {
				continue
			}
			if embedsRiverWorkerDefaults(field.Type) {
				return true
			}
		}
	case *ast.SelectorExpr:
		pkg, ok := typed.X.(*ast.Ident)
		return ok && pkg.Name == "river" && typed.Sel.Name == "WorkerDefaults"
	case *ast.IndexExpr:
		return embedsRiverWorkerDefaults(typed.X)
	case *ast.IndexListExpr:
		return embedsRiverWorkerDefaults(typed.X)
	case *ast.StarExpr:
		return embedsRiverWorkerDefaults(typed.X)
	}
	return false
}

func isKindMethod(fn *ast.FuncDecl) bool {
	if fn.Type.Params != nil && len(fn.Type.Params.List) > 0 {
		return false
	}
	results := fn.Type.Results
	if results == nil || len(results.List) != 1 {
		return false
	}
	ident, ok := results.List[0].Type.(*ast.Ident)
	return ok && ident.Name == "string"
}

func readOnlyListCommand(use, short, dir, ext string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}
			items, err := listProjectFiles(rootDir, dir, ext, strings.TrimSuffix(use, "s"))
			if err != nil {
				return err
			}
			return output.OK(cmd, items, short)
		},
	}
	setAgentMetadata(cmd, "introspection", "Read-only project shape inspection.")
	return cmd
}

// excludeDomainInternalFiles removes generated output under domains/internal/
// from the inspect domains item list. Those files are emitted by
// `andurel sync queries` and belong to the internal queries package, so they
// are not domain entities.
func excludeDomainInternalFiles(items []projectItem) []projectItem {
	filtered := make([]projectItem, 0, len(items))
	for _, item := range items {
		if isDomainInternalFile(item.Path) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func listProjectFiles(rootDir, relDir, ext, kind string) ([]projectItem, error) {
	base := filepath.Join(rootDir, relDir)
	if _, err := os.Stat(base); err != nil {
		return []projectItem{}, nil
	}
	items := []projectItem{}
	err := filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if ext != "" && filepath.Ext(path) != ext {
			return nil
		}
		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		items = append(items, projectItem{
			Name: name,
			Path: filepath.ToSlash(rel),
			Kind: kind,
		})
		return nil
	})
	sortProjectItems(items)
	return items, err
}

func sortProjectItems(items []projectItem) {
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Path < items[j].Path
	})
}

func readGoModMetadata(rootDir string) (module, goVersion string, err error) {
	data, err := os.ReadFile(filepath.Join(rootDir, "go.mod"))
	if err != nil {
		return "", "", err
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "module":
			module = fields[1]
		case "go":
			goVersion = fields[1]
		}
	}
	return module, goVersion, nil
}

func toolInfos(rootDir string, lock *layout.AndurelLock) []toolInfo {
	infos := []toolInfo{}
	if lock == nil {
		return infos
	}
	for name, tool := range lock.Tools {
		info := toolInfo{Name: name}
		if tool != nil {
			info.Version = tool.Version
			info.Source = tool.Source
			info.Path = tool.Path
		}
		if info.Path != "" {
			info.BinaryPath = info.Path
		} else {
			info.BinaryPath = filepath.ToSlash(filepath.Join("bin", name))
		}
		if _, err := os.Stat(
			filepath.Join(rootDir, filepath.FromSlash(info.BinaryPath)),
		); err == nil {
			info.Installed = true
		}
		infos = append(infos, info)
	}
	sort.SliceStable(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos
}
