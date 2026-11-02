package routing

import (
	"fmt"
	"maps"
	"net"
	"strconv"
	"strings"
	"unicode"
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
	index map[string]HostName
}

// NewHosts validates and clones a host registry. HostPrimary must be present
// with a non-empty hostname. Every other entry must also have a hostname.
// Hostnames and aliases must be an authority: host or host:port, with an IPv6
// literal in brackets. A scheme, path, query, fragment, user info, whitespace,
// an empty host part or a bad port is rejected. Hostname and alias strings
// must be unique across HostNames; uniqueness is case-insensitive because
// hostnames are.
func NewHosts(specs map[HostName]HostSpec) (*Hosts, error) {
	if len(specs) == 0 {
		return nil, fmt.Errorf("routing: host registry must include %q", HostPrimary)
	}
	cloned := make(map[HostName]HostSpec, len(specs))
	claimed := make(map[string]HostName)
	index := make(map[string]HostName)
	claim := func(hostname string, name HostName) error {
		hostname = strings.TrimSpace(hostname)
		if hostname == "" {
			return nil
		}
		key := strings.ToLower(hostname)
		if other, ok := claimed[key]; ok && other != name {
			return fmt.Errorf(
				"routing: hostname %q is claimed by both %q and %q",
				hostname,
				other,
				name,
			)
		}
		claimed[key] = name
		index[key] = name
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
		if err := validateAuthority(spec.Hostname); err != nil {
			return nil, fmt.Errorf(
				"routing: host %q (%s): hostname %q %w",
				name,
				hostEnvKey(name),
				spec.Hostname,
				err,
			)
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
				if err := validateAuthority(alias); err != nil {
					return nil, fmt.Errorf(
						"routing: host %q: alias %q %w",
						name,
						alias,
						err,
					)
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

	return &Hosts{specs: cloned, index: index}, nil
}

// validateAuthority reports why value is not a hostname or hostname:port. The
// error completes a sentence that starts with the offending value.
func validateAuthority(value string) error {
	const form = "must be host or host:port"
	if strings.ContainsFunc(value, unicode.IsSpace) {
		return fmt.Errorf("%s: it contains whitespace", form)
	}
	if strings.Contains(value, "://") {
		return fmt.Errorf("%s: remove the scheme", form)
	}
	if strings.ContainsAny(value, "/\\?#") {
		return fmt.Errorf("%s: remove the path", form)
	}
	if strings.Contains(value, "@") {
		return fmt.Errorf("%s: remove the user info", form)
	}

	host, port := value, ""
	switch {
	case strings.HasPrefix(value, "["):
		end := strings.IndexByte(value, ']')
		if end < 0 {
			return fmt.Errorf("%s: the IPv6 address has no closing bracket", form)
		}
		host = value[1:end]
		rest := value[end+1:]
		if rest != "" {
			if !strings.HasPrefix(rest, ":") {
				return fmt.Errorf("%s: unexpected %q after the IPv6 address", form, rest)
			}
			port = rest[1:]
			if port == "" {
				return fmt.Errorf("%s: the port is empty", form)
			}
		}
		if host == "" {
			return fmt.Errorf("%s: the host part is empty", form)
		}
		if net.ParseIP(host) == nil || !strings.Contains(host, ":") {
			return fmt.Errorf("%s: %q in brackets is not an IPv6 address", form, host)
		}
	default:
		switch strings.Count(value, ":") {
		case 0:
		case 1:
			var cut bool
			host, port, cut = strings.Cut(value, ":")
			if cut && port == "" {
				return fmt.Errorf("%s: the port is empty", form)
			}
		default:
			return fmt.Errorf("%s: put an IPv6 address in brackets", form)
		}
		if host == "" {
			return fmt.Errorf("%s: the host part is empty", form)
		}
		for _, r := range host {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '.' || r == '_' {
				continue
			}
			return fmt.Errorf("%s: it contains %q", form, r)
		}
	}
	if port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 ||
			strings.ContainsFunc(port, func(r rune) bool { return r < '0' || r > '9' }) {
			return fmt.Errorf("%s: port %q is not a number from 1 to 65535", form, port)
		}
	}
	return nil
}

// hostEnvKey names the environment variable the generated config reads a
// host's hostname from: HOST_ followed by the name in upper case, with every
// character other than a letter or digit replaced by an underscore.
func hostEnvKey(name HostName) string {
	return "HOST_" + strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r):
			return unicode.ToUpper(r)
		case unicode.IsDigit(r):
			return r
		default:
			return '_'
		}
	}, string(name))
}

// Resolve maps an HTTP authority (Request.Host: a hostname, hostname:port, or
// bracketed IPv6 literal) to its configured HostName. Matching is
// case-insensitive. An exact configured hostname or alias wins; otherwise the
// port is stripped and the bare host is matched, so a configured bare hostname
// claims every port, not only :80 and :443. Unknown authorities return
// ("", false). A nil *Hosts resolves nothing.
func (h *Hosts) Resolve(authority string) (HostName, bool) {
	if h == nil {
		return "", false
	}
	authority = strings.ToLower(strings.TrimSpace(authority))
	if authority == "" {
		return "", false
	}
	if name, ok := h.index[authority]; ok {
		return name, true
	}
	host, _, err := net.SplitHostPort(authority)
	if err != nil {
		return "", false
	}
	if name, ok := h.index[host]; ok {
		return name, true
	}
	// net.SplitHostPort strips the brackets from an IPv6 literal while the
	// registry stores configured hostnames in URL authority form.
	if strings.Contains(host, ":") {
		if name, ok := h.index["["+host+"]"]; ok {
			return name, true
		}
	}
	return "", false
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
