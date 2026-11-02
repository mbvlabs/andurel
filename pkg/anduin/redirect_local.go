package anduin

import (
	"context"
	"net/url"
	"strings"
)

// SafeLocalPath reports whether target is a path on this site that is safe to
// redirect to, such as "/users" or "/users?tab=1". It checks the target the way
// browsers read it: leading and trailing ASCII whitespace and control
// characters are removed, tabs, newlines and carriage returns are removed
// anywhere, and backslashes count as slashes. So "/\t/evil.example" and
// "/\evil.example" are rejected, because browsers read both as
// "//evil.example", which is another site.
//
// The target must start with a single "/" after that normalisation, and it must
// parse as a URL with no scheme and no host. An empty target is not safe.
func SafeLocalPath(target string) bool {
	_, ok := localPath(target)
	return ok
}

// RedirectLocal redirects to target when SafeLocalPath accepts it, and to
// fallback otherwise. The redirect uses the normalised form of target, so the
// location the browser receives is the one that was checked. status must be a
// 3xx code, as with Redirect.
func RedirectLocal(ctx context.Context, target, fallback string, status int) error {
	if location, ok := localPath(target); ok {
		return Redirect(ctx, location, status)
	}
	return Redirect(ctx, fallback, status)
}

// localPath returns the normalised target and whether it is a safe local path.
func localPath(target string) (string, bool) {
	normalised := normaliseLocalPath(target)
	if !strings.HasPrefix(normalised, "/") || strings.HasPrefix(normalised, "//") {
		return "", false
	}
	parsed, err := url.Parse(normalised)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || parsed.User != nil {
		return "", false
	}
	return normalised, true
}

// normaliseLocalPath applies the browser rules that matter for a redirect
// target: trim leading and trailing ASCII whitespace and control characters,
// drop tabs, newlines and carriage returns, and read backslashes as slashes.
func normaliseLocalPath(target string) string {
	target = strings.TrimFunc(target, func(r rune) bool { return r <= ' ' })
	target = strings.NewReplacer("\t", "", "\n", "", "\r", "").Replace(target)
	return strings.ReplaceAll(target, `\`, "/")
}
