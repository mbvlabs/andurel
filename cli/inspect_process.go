package cli

import (
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/spf13/cobra"
)

type processInspection struct {
	Processes []processInfo `json:"processes"`
}

type processInfo struct {
	Name                string   `json:"name"`
	Source              string   `json:"source"`
	Buildable           bool     `json:"buildable"`
	Responsibility      string   `json:"responsibility,omitempty"`
	ResponsibilityKnown bool     `json:"responsibility_known"`
	Prerequisites       []string `json:"prerequisites"`
	ConfigKeys          []string `json:"config_keys"`
	StartupOrder        int      `json:"startup_order"`
	StartupNote         string   `json:"startup_note,omitempty"`
	HealthEndpoint      string   `json:"health_endpoint,omitempty"`
	HealthResolved      bool     `json:"health_resolved"`
	HealthReason        string   `json:"health_reason,omitempty"`
}

type processDefinition struct {
	responsibility string
	prerequisites  []string
	configKeys     []string
	startupOrder   int
	startupNote    string
}

var processDefinitions = map[string]processDefinition{
	"app": {
		responsibility: "Serves HTTP, renders Inertia pages, embeds client assets, and answers health checks",
		prerequisites: []string{
			"Production build (dist/app)",
			"Reachable PostgreSQL database",
			"Healthy SSR sidecar when Inertia SSR is enabled",
		},
		configKeys:   []string{"DB_*", "HOST_PRIMARY", "PROTOCOL", "SESSION_*", "EMAIL_*", "CACHE_*", "QUEUE_*", "TELEMETRY_*", "INERTIA_*"},
		startupOrder: 3,
		startupNote:  "Start after migrate and a healthy ssr sidecar; runs beside queue.",
	},
	"queue": {
		responsibility: "Runs River jobs against PostgreSQL; no HTTP listener",
		prerequisites: []string{
			"Production build (dist/queue)",
			"Reachable PostgreSQL database",
		},
		configKeys:   []string{"DB_*", "QUEUE_*", "EMAIL_*", "CACHE_*", "TELEMETRY_*"},
		startupOrder: 3,
		startupNote:  "Start after migrate; runs beside app with the same environment.",
	},
	"migrate": {
		responsibility: "Applies the embedded SQL migrations once and exits",
		prerequisites: []string{
			"Production build (dist/migrate)",
			"Reachable PostgreSQL database",
		},
		configKeys:   []string{"DB_*"},
		startupOrder: 1,
		startupNote:  "Run once before ssr, app, and queue; never from application startup.",
	},
	"ssr": {
		responsibility: "Runs the Inertia SSR bundle and answers /health and /render",
		prerequisites: []string{
			"Production build (dist/ssr) with the embedded SSR bundle",
			"JavaScript runtime (node) major version 22 or newer on PATH",
		},
		configKeys:   []string{"INERTIA_*", "PATH"},
		startupOrder: 2,
		startupNote:  "Start after migrate and before app; wait for GET /health.",
	},
	"seeds": {
		responsibility: "Runs registered database seed sets through an explicit command",
		prerequisites: []string{
			"Applied migrations",
			"Reachable PostgreSQL database",
		},
		configKeys:   []string{"DB_*"},
		startupOrder: 0,
		startupNote:  "Manual one-shot step; run andurel db seed after migrations when a seed set is required.",
	},
}

func newProcessCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "process",
		Short: "List runnable cmd processes, responsibilities, and health endpoints",
		Long: `List runnable processes under cmd/ without compiling or starting anything.

Each entry reports the source main file, whether andurel build produces it,
its documented responsibility, configuration needs, expected startup order,
and its health endpoint when the project declares one. Processes without a
documented definition keep responsibility_known=false and an unresolved
health endpoint instead of being guessed.`,
		Args:    cobra.NoArgs,
		Example: "  andurel inspect process\n  andurel inspect process --json\n  andurel inspect process --jq .processes",
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}
			report, err := collectProcessesInspection(rootDir)
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
				return renderProcessesHuman(cmd.OutOrStdout(), report)
			}
			return output.OK(
				cmd,
				report,
				fmt.Sprintf("Listed %d runnable processes", len(report.Processes)),
			)
		},
	}
	setAgentMetadata(
		cmd,
		"introspection",
		"Read-only process inventory with source, responsibility, config needs, startup order, and health endpoints.",
	)
	return cmd
}

func collectProcessesInspection(rootDir string) (processInspection, error) {
	processes, err := discoverInspectProcesses(rootDir)
	if err != nil {
		return processInspection{}, err
	}
	manifest, err := collectRouteManifest(rootDir)
	if err != nil {
		return processInspection{}, err
	}
	resolver := newProjectEnvResolver(rootDir)

	for i := range processes {
		definition, known := processDefinitions[processes[i].Name]
		processes[i].ResponsibilityKnown = known
		if known {
			processes[i].Responsibility = definition.responsibility
			processes[i].Prerequisites = append(
				[]string(nil),
				definition.prerequisites...,
			)
			processes[i].ConfigKeys = append([]string(nil), definition.configKeys...)
			processes[i].StartupOrder = definition.startupOrder
			processes[i].StartupNote = definition.startupNote
		} else {
			processes[i].Prerequisites = []string{}
			processes[i].ConfigKeys = []string{}
			processes[i].StartupNote = "No documented responsibility and startup contract for this process name."
		}
		applyProcessHealth(&processes[i], manifest, resolver)
	}
	return processInspection{Processes: processes}, nil
}

func applyProcessHealth(
	process *processInfo,
	manifest routeManifest,
	resolver projectEnvResolver,
) {
	process.HealthEndpoint = ""
	process.HealthResolved = false
	process.HealthReason = ""
	switch process.Name {
	case "app":
		for _, route := range manifest.Routes {
			if route.Name == "api.health" || route.Path == "/api/health" {
				process.HealthEndpoint = route.Path
				process.HealthResolved = true
				process.HealthReason = "GET " + route.Path + " (route " + route.Name + ")"
				return
			}
		}
		process.HealthReason = "no api.health route is declared in the route manifest"
	case "ssr":
		listen := resolver.resolve("INERTIA_SSR_LISTEN")
		if !listen.Found || strings.TrimSpace(listen.Value) == "" {
			listen = resolver.resolve("INERTIA_SSR_URL")
		}
		if !listen.Found || strings.TrimSpace(listen.Value) == "" {
			process.HealthReason = listen.describe()
			return
		}
		parsed, err := url.Parse(strings.TrimSpace(listen.Value))
		if err != nil || parsed.Host == "" ||
			(parsed.Scheme != "http" && parsed.Scheme != "https") {
			process.HealthReason = fmt.Sprintf(
				"%s value %q is not an HTTP or HTTPS URL",
				listen.Key,
				listen.Value,
			)
			return
		}
		parsed.Path = strings.TrimSuffix(strings.TrimSuffix(parsed.Path, "/render"), "/")
		parsed.RawQuery = ""
		parsed.Fragment = ""
		process.HealthEndpoint = strings.TrimSuffix(parsed.String(), "/") + "/health"
		process.HealthResolved = true
		process.HealthReason = "GET /health on the SSR listen address (" + listen.describe() + ")"
	case "queue":
		process.HealthResolved = true
		process.HealthReason = "queue exposes no HTTP endpoint; supervise its process exit status"
	case "migrate", "seeds":
		process.HealthResolved = true
		process.HealthReason = "one-shot process; success is exit status 0"
	default:
		process.HealthReason = "no health endpoint is documented for this process"
	}
}

func discoverInspectProcesses(rootDir string) ([]processInfo, error) {
	cmdDir := filepath.Join(rootDir, "cmd")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []processInfo{}, nil
		}
		return nil, err
	}
	processes := make([]processInfo, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(cmdDir, entry.Name())
		hasMain, err := hasMainPackage(dir)
		if err != nil {
			return nil, err
		}
		if !hasMain {
			continue
		}
		source, ok, err := mainPackageFile(dir)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		rel, err := filepath.Rel(rootDir, source)
		if err != nil {
			rel = source
		}
		processes = append(processes, processInfo{
			Name:      entry.Name(),
			Source:    filepath.ToSlash(rel),
			Buildable: entry.Name() != "seeds",
		})
	}
	sort.SliceStable(processes, func(i, j int) bool {
		return processes[i].Name < processes[j].Name
	})
	return processes, nil
}

func mainPackageFile(dir string) (string, bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false, err
	}
	var found string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.PackageClauseOnly)
		if err != nil {
			return "", false, fmt.Errorf("parse %s: %w", path, err)
		}
		if file.Name.Name == "main" {
			if entry.Name() == "main.go" {
				return path, true, nil
			}
			if found == "" {
				found = path
			}
		}
	}
	if found == "" {
		return "", false, nil
	}
	return found, true, nil
}

func renderProcessesHuman(w io.Writer, report processInspection) error {
	if len(report.Processes) == 0 {
		_, err := fmt.Fprintln(w, "No runnable processes found under cmd/.")
		return err
	}
	table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "PROCESS\tSOURCE\tORDER\tHEALTH\tRESPONSIBILITY"); err != nil {
		return err
	}
	for _, process := range report.Processes {
		health := process.HealthEndpoint
		if health == "" {
			health = "-"
		}
		responsibility := process.Responsibility
		if responsibility == "" {
			responsibility = "not documented"
		}
		if _, err := fmt.Fprintf(
			table,
			"%s\t%s\t%d\t%s\t%s\n",
			process.Name,
			process.Source,
			process.StartupOrder,
			health,
			responsibility,
		); err != nil {
			return err
		}
	}
	return table.Flush()
}
