package routing

import (
	"fmt"
	"maps"
	"strings"
	"sync"
)

// HostName identifies a named virtual host. Framework code owns the type and
// HostPrimary; applications declare additional names (for example
// `const HostAdmin routing.HostName = "admin"`).
type HostName string

// HostPrimary is the required default host. Unknown Request.Host values are
// served by the primary Echo.
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

var (
	hostMu       sync.RWMutex
	hostRegistry map[HostName]HostSpec
)

// ConfigureHosts installs the boot-time hostname registry used by FullURL.
// HostPrimary must be present with a non-empty hostname. Every other entry
// must also have a hostname. Calling again replaces the registry.
func ConfigureHosts(hosts map[HostName]HostSpec) error {
	if len(hosts) == 0 {
		return fmt.Errorf("routing: host registry must include %q", HostPrimary)
	}
	cloned := make(map[HostName]HostSpec, len(hosts))
	for name, spec := range hosts {
		if strings.TrimSpace(string(name)) == "" {
			return fmt.Errorf("routing: host name must not be empty")
		}
		spec.Hostname = strings.TrimSpace(spec.Hostname)
		if spec.Hostname == "" {
			return fmt.Errorf("routing: host %q is missing a hostname", name)
		}
		if len(spec.Aliases) > 0 {
			aliases := make([]string, 0, len(spec.Aliases))
			for _, alias := range spec.Aliases {
				alias = strings.TrimSpace(alias)
				if alias == "" {
					continue
				}
				aliases = append(aliases, alias)
			}
			spec.Aliases = aliases
		}
		cloned[name] = spec
	}
	primary, ok := cloned[HostPrimary]
	if !ok || primary.Hostname == "" {
		return fmt.Errorf("routing: host %q is required", HostPrimary)
	}

	hostMu.Lock()
	hostRegistry = cloned
	hostMu.Unlock()
	return nil
}

// LookupHost returns the registered spec for name.
func LookupHost(name HostName) (HostSpec, bool) {
	hostMu.RLock()
	defer hostMu.RUnlock()
	if hostRegistry == nil {
		return HostSpec{}, false
	}
	spec, ok := hostRegistry[name]
	return spec, ok
}

// Hosts returns a copy of the boot-time hostname registry.
func Hosts() map[HostName]HostSpec {
	hostMu.RLock()
	defer hostMu.RUnlock()
	if hostRegistry == nil {
		return nil
	}
	return maps.Clone(hostRegistry)
}

// HostBaseURL returns the origin for name from the boot-time registry.
// Unregistered names fall back to HostPrimary. An unconfigured registry
// returns the empty string.
func HostBaseURL(name HostName) string {
	if name == "" {
		name = HostPrimary
	}
	if spec, ok := LookupHost(name); ok {
		return spec.BaseURL()
	}
	if name != HostPrimary {
		if spec, ok := LookupHost(HostPrimary); ok {
			return spec.BaseURL()
		}
	}
	return ""
}

func routeHost(cfg routeConfig) HostName {
	if cfg.host == "" {
		return HostPrimary
	}
	return cfg.host
}

func (r Route) fullURL(path string) string {
	return HostBaseURL(r.host) + path
}
