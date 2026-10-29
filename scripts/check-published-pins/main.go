// Command check-published-pins verifies that every standalone pkg/* pin in
// layout/versions is published and resolvable from the public Go module proxy.
//
// It is the release gate that a local module proxy cannot satisfy. Exit status
// is non-zero if any pin fails its checksum lookup, proxy lookup, resolution, or
// cache verification.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/mbvlabs/andurel/v2/internal/publishedpins"
)

func main() {
	root := flag.String("root", ".", "repository root")
	timeout := flag.Duration("timeout", 10*time.Minute, "overall check timeout")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	checker := publishedpins.New(publishedpins.Options{Root: *root})
	results, err := checker.Check(ctx)
	for _, result := range results {
		if result.SumDB && result.Proxy && result.Resolved && result.Checksum {
			fmt.Printf("ok   %s\n", result.Pin)
			continue
		}
		fmt.Printf(
			"FAIL %s (sumdb=%t proxy=%t resolved=%t checksum=%t)\n",
			result.Pin,
			result.SumDB,
			result.Proxy,
			result.Resolved,
			result.Checksum,
		)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "published-pin check failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%d published pkg/* pins verified\n", len(results))
}
