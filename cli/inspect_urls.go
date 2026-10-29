package cli

import (
	"fmt"
	"io"
	"net/url"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/spf13/cobra"
)

type urlRoute struct {
	Name             string   `json:"name"`
	Variable         string   `json:"variable"`
	Methods          []string `json:"methods"`
	MethodsResolved  bool     `json:"methods_resolved"`
	Path             string   `json:"path"`
	Pattern          string   `json:"pattern,omitempty"`
	Host             string   `json:"host"`
	HostConfigKey    string   `json:"host_config_key,omitempty"`
	Origin           string   `json:"origin,omitempty"`
	HostSource       string   `json:"host_source,omitempty"`
	ProtocolSource   string   `json:"protocol_source,omitempty"`
	OriginSource     string   `json:"origin_source,omitempty"`
	URL              string   `json:"url,omitempty"`
	URLTemplate      string   `json:"url_template,omitempty"`
	Dynamic          bool     `json:"dynamic,omitempty"`
	Resolved         bool     `json:"resolved"`
	UnresolvedReason string   `json:"unresolved_reason,omitempty"`
}

type urlsInspection struct {
	Routes []urlRoute `json:"routes"`
}

func newURLsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "urls",
		Short: "List routes with resolved host origins and full URLs",
		Long: `List route names, methods, paths, host keys, resolved origins, and full URLs.

Origins come from the application's declared configuration: HOST_PRIMARY,
HOST_<NAME> for extra hosts, and PROTOCOL. Values are read from exported
environment variables first, then .env, then config/*.go defaults. A route
whose host has no configured value is reported with resolved=false and a
reason; no localhost URL is invented. Dynamic route parameters are reported
as url_template with the :param segments intact.`,
		Args:    cobra.NoArgs,
		Example: "  andurel inspect urls\n  andurel inspect urls --json\n  andurel inspect urls --jq .routes",
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}
			report, err := collectURLsInspection(rootDir)
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
				return renderURLsHuman(cmd.OutOrStdout(), report)
			}
			return output.OK(
				cmd,
				report,
				fmt.Sprintf("Listed %d routes with URL configuration", len(report.Routes)),
			)
		},
	}
	setAgentMetadata(
		cmd,
		"introspection",
		"Read-only route URL view with host config provenance; unresolved hosts are explicit, not invented.",
	)
	return cmd
}

func collectURLsInspection(rootDir string) (urlsInspection, error) {
	manifest, err := collectRouteManifest(rootDir)
	if err != nil {
		return urlsInspection{}, err
	}
	methodsByVariable, err := routeMethodsByVariable(rootDir, manifest)
	if err != nil {
		return urlsInspection{}, err
	}
	resolver := newProjectEnvResolver(rootDir)
	hostNames, err := discoverAppHostNames(rootDir)
	if err != nil {
		return urlsInspection{}, err
	}
	discoveredHosts := make(map[string]discoveredHost, len(hostNames))
	for _, host := range hostNames {
		discoveredHosts[host.Value] = host
	}

	routes := make([]urlRoute, 0, len(manifest.Routes))
	for _, route := range manifest.Routes {
		methodSet := methodsByVariable[route.Variable]
		methods := methodSet.methods
		methodsResolved := methodSet.resolved
		entry := urlRoute{
			Name:            route.Name,
			Variable:        route.Variable,
			Methods:         methods,
			MethodsResolved: methodsResolved,
			Path:            route.Path,
			Pattern:         route.Pattern,
			Host:            route.Host,
			Dynamic:         len(route.Params) > 0,
		}
		if entry.Methods == nil {
			entry.Methods = []string{}
		}

		hostValue := route.Host
		envKey := hostConfigKey(hostValue)
		entry.HostConfigKey = envKey
		hostSource := resolver.resolve(envKey)
		protocolSource := resolver.protocol()
		entry.HostSource = hostSource.describe()
		entry.ProtocolSource = protocolSource.describe()

		host := strings.TrimSpace(hostSource.Value)
		protocol := strings.TrimSpace(protocolSource.Value)
		switch {
		case !hostSource.Found || host == "":
			entry.UnresolvedReason = hostSource.describe()
		case protocol != "http" && protocol != "https":
			entry.UnresolvedReason = fmt.Sprintf(
				"PROTOCOL value %q is not http or https",
				protocol,
			)
		default:
			origin := protocol + "://" + host
			parsed, err := url.Parse(origin)
			if err != nil || parsed.Host == "" {
				entry.UnresolvedReason = fmt.Sprintf("host value %q does not form a valid origin", host)
				break
			}
			entry.Origin = origin
			entry.OriginSource = originSourceSummary(hostSource, protocolSource)
			entry.URLTemplate = origin + route.Path
			entry.Resolved = true
			if !entry.Dynamic {
				entry.URL = entry.URLTemplate
			}
		}
		if discovered, ok := discoveredHosts[hostValue]; ok && entry.HostConfigKey != "" {
			entry.HostSource = entry.HostSource + " (declared as " + discovered.GoExpr + ")"
		}
		routes = append(routes, entry)
	}
	return urlsInspection{Routes: routes}, nil
}

func originSourceSummary(hostSource, protocolSource configValueSource) string {
	parts := []string{hostSource.describe(), protocolSource.describe()}
	return strings.Join(parts, "; ")
}

func routeMethodsByVariable(
	rootDir string,
	manifest routeManifest,
) (map[string]routeMethodSet, error) {
	items, err := listProjectFiles(rootDir, "handlers", ".go", "handler")
	if err != nil {
		return nil, err
	}
	module, _, err := readGoModMetadata(rootDir)
	if err != nil {
		return nil, err
	}
	items = describeHandlerItems(rootDir, module, items, manifest)
	methods := map[string]routeMethodSet{}
	for _, item := range items {
		for _, handler := range item.Handlers {
			for _, route := range handler.Routes {
				if route.Variable == "" {
					continue
				}
				set := methods[route.Variable]
				set.add(route.Method)
				methods[route.Variable] = set
			}
		}
	}
	return methods, nil
}

type routeMethodSet struct {
	methods  []string
	resolved bool
}

func (set *routeMethodSet) add(method string) {
	if !slices.Contains(set.methods, method) {
		set.methods = append(set.methods, method)
	}
	set.resolved = true
}

func renderURLsHuman(w io.Writer, report urlsInspection) error {
	if len(report.Routes) == 0 {
		_, err := fmt.Fprintln(w, "No routes found.")
		return err
	}
	table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "ROUTE\tMETHOD\tHOST\tPATH\tURL"); err != nil {
		return err
	}
	unresolved := make([]urlRoute, 0)
	for _, route := range report.Routes {
		method := strings.Join(route.Methods, ",")
		if method == "" {
			method = "-"
		}
		address := route.URL
		if address == "" {
			address = "-"
		}
		if _, err := fmt.Fprintf(
			table,
			"%s\t%s\t%s\t%s\t%s\n",
			route.Name,
			method,
			route.Host,
			route.Path,
			address,
		); err != nil {
			return err
		}
		if !route.Resolved {
			unresolved = append(unresolved, route)
		}
	}
	if err := table.Flush(); err != nil {
		return err
	}
	if len(unresolved) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w, "Unresolved hosts:")
	if err != nil {
		return err
	}
	for _, route := range unresolved {
		if _, err := fmt.Fprintf(w, "  %s: %s\n", route.Name, route.UnresolvedReason); err != nil {
			return err
		}
	}
	return nil
}
