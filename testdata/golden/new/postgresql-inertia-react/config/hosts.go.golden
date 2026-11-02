package config

import (
	"strings"
	"unicode"

	"github.com/mbvlabs/andurel/pkg/routing"
)

// extraHostNames lists the hosts this app serves besides HostPrimary. Declare
// each name as a routing.HostName constant, then list it here, for example:
//
//	const HostAdmin routing.HostName = "admin"
//
//	var extraHostNames = []routing.HostName{HostAdmin}
//
// NewApp reads HOST_<NAME> for each listed name (HOST_ADMIN for HostAdmin). When
// the variable is unset the host defaults to <name>. followed by HOST_PRIMARY,
// for example admin.example.com. Every host is validated with the rest of the
// config. Bind routes to a name with routing.Host(HostAdmin) and register them
// with rt.Route.
var extraHostNames []routing.HostName

// hostEnvKey returns the environment key for a host name: HOST_ followed by the
// name in upper case, with every other character replaced by an underscore.
// andurel inspect urls derives the same key.
func hostEnvKey(name routing.HostName) string {
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

// defaultHostname is the hostname a listed host uses when its HOST_<NAME>
// variable is unset: the name in front of the primary hostname.
func defaultHostname(name routing.HostName, primaryHost string) string {
	return string(name) + "." + primaryHost
}
