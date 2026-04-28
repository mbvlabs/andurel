// Command andurel-sqlc-gen is the andurel sqlc process plugin.
//
// It is invoked by sqlc when listed as a `process:` plugin in sqlc.yaml.
// sqlc passes a GenerateRequest on stdin and reads a GenerateResponse on stdout;
// arguments and protocol are handled by the plugin SDK in internal/sqlcgen.
//
// Direct invocation is not useful — the binary expects sqlc's RPC framing
// and exits with a usage error otherwise.
package main

import "github.com/mbvlabs/andurel/internal/sqlcgen"

func main() {
	sqlcgen.Run()
}
