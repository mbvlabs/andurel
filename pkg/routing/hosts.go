package routing

import (
	"fmt"
	"maps"
	"strings"
)

// HostName identifies a named virtual host. Framework code owns the type and
// HostPrimary; applications declare additional names (for example
// `const HostAdmin routing.HostName = "admin"`).
type HostName string

// HostPrimary is the required default host. Unknown Request.Host values are
// served by the primary router.
const HostPrimary HostName = "primary"

// HostSpec is the public hostname (and optional aliases) for a HostName.
type HostSpec struct {
	Hostname string
	Aliases  []string
	Protocol string
}

// BaseURL returns the origin for this host spec (protocol + hostname).
func (s HostSpec) BaseURL() string {
	return composeBaseURL(s.Protocol, s.Hostname)
}

// Origins returns the unique origins for the canonical hostname and aliases.
func (s HostSpec) Origins() []string {
	origins := make([]string, 0, 1+len(s.Aliases))
	if origin := composeBaseURL(s.Protocol, s.Hostname); origin != "" {
		origins = append(origins, origin)
	}
	seen := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		seen[origin] = struct{}{}
	}
	for _, alias := range s.Aliases {
		origin := composeBaseURL(s.Protocol, alias)
		if origin == "" {
			continue
		}
		if _, ok := seen[origin]; ok {
			continue
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}
	return origins
}

func composeBaseURL(protocol, hostname string) string {
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		return ""
	}
	protocol = strings.TrimSpace(protocol)
	if protocol == "" {
		protocol = "http"
	}
	return protocol + "://" + hostname
}

// Host binds a route to a named host. Omitting it selects HostPrimary.
func Host(name HostName) RouteSetupOption {
	return func(cfg *routeConfig) {
		if name != "" {
			cfg.host = name
		}
	}
}

// Hosts is an immutable hostname registry used by route URLs and the router.
type Hosts struct {
	specs map[HostName]HostSpec
}

// NewHosts validates and clones a host registry. HostPrimary must be present
// with a non-empty hostname. Every other entry must also have a hostname.
// Hostname and alias strings must be unique across HostNames.
func NewHosts(specs map[HostName]HostSpec) (*Hosts, error) {
	if len(specs) == 0 {
		return nil, fmt.Errorf("routing: host registry must include %q", HostPrimary)
	}
	cloned := make(map[HostName]HostSpec, len(specs))
	claimed := make(map[string]HostName)
	claim := func(hostname string, name HostName) error {
		hostname = strings.TrimSpace(hostname)
		if hostname == "" {
			return nil
		}
		if other, ok := claimed[hostname]; ok && other != name {
			return fmt.Errorf(
				"routing: hostname %q is claimed by both %q and %q",
				hostname,
				other,
				name,
			)
		}
		claimed[hostname] = name
		return nil
	}
	for name, spec := range specs {
		if strings.TrimSpace(string(name)) == "" {
			return nil, fmt.Errorf("routing: host name must not be empty")
		}
		spec.Hostname = strings.TrimSpace(spec.Hostname)
		if spec.Hostname == "" {
			return nil, fmt.Errorf("routing: host %q is missing a hostname", name)
		}
		if err := claim(spec.Hostname, name); err != nil {
			return nil, err
		}
		if len(spec.Aliases) > 0 {
			aliases := make([]string, 0, len(spec.Aliases))
			for _, alias := range spec.Aliases {
				alias = strings.TrimSpace(alias)
				if alias == "" {
					continue
				}
				if err := claim(alias, name); err != nil {
					return nil, err
				}
				aliases = append(aliases, alias)
			}
			spec.Aliases = aliases
		}
		cloned[name] = spec
	}
	primary, ok := cloned[HostPrimary]
	if !ok || primary.Hostname == "" {
		return nil, fmt.Errorf("routing: host %q is required", HostPrimary)
	}

	return &Hosts{specs: cloned}, nil
}

// Lookup returns the registered spec for name.
func (h *Hosts) Lookup(name HostName) (HostSpec, bool) {
	if h == nil {
		return HostSpec{}, false
	}
	spec, ok := h.specs[name]
	return spec, ok
}

// All returns a copy of the host registry.
func (h *Hosts) All() map[HostName]HostSpec {
	if h == nil {
		return nil
	}
	return maps.Clone(h.specs)
}

// BaseURL returns the origin for name. Empty name selects HostPrimary.
// Unregistered names return the empty string (no silent fallback to another
// host).
func (h *Hosts) BaseURL(name HostName) string {
	if name == "" {
		name = HostPrimary
	}
	if spec, ok := h.Lookup(name); ok {
		return spec.BaseURL()
	}
	return ""
}

func routeHost(cfg routeConfig) HostName {
	if cfg.host == "" {
		return HostPrimary
	}
	return cfg.host
}

func (r Route) fullURL(hosts *Hosts, path string) string {
	return hosts.BaseURL(r.host) + path
}
