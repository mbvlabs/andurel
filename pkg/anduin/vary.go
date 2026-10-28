package anduin

import (
	"net/http"
	"strings"
)

// AddVary appends tokens to the Vary response header without duplicating a
// field name the header already lists. Existing comma-separated values are
// compared case-insensitively after trimming, so the first-seen spelling is
// kept. "*" is an ordinary token, not a wildcard.
func AddVary(header http.Header, tokens ...string) {
	if header == nil {
		return
	}

	seen := make(map[string]struct{})
	for _, line := range header.Values("Vary") {
		for item := range strings.SplitSeq(line, ",") {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			seen[strings.ToLower(item)] = struct{}{}
		}
	}

	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}

		key := strings.ToLower(token)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		header.Add("Vary", token)
	}
}
