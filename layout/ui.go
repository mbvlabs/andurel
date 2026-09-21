package layout

import (
	"fmt"
	"strings"
)

const (
	DefaultUICombo   = "react/pnpm"
	DatastarUICombo  = "templ/datastar"
	DefaultUIAdapter = "react"
	DefaultUIPackage = "pnpm"
)

// UISelection is the parsed result of an andurel new --ui value.
type UISelection struct {
	// Inertia is the adapter (react, vue, svelte). Empty means Datastar/templ UI.
	Inertia string
	// PackageManager is the JS package manager. Empty for Datastar UI.
	PackageManager string
}

// IsInertia reports whether the selection uses Inertia.
func (s UISelection) IsInertia() bool {
	return IsSupportedInertiaAdapter(s.Inertia)
}

// IsDatastar reports whether the selection uses templ + Datastar.
func (s UISelection) IsDatastar() bool {
	return s.Inertia == ""
}

// ParseUICombo parses a compound --ui value such as "react/pnpm" or "templ/datastar".
// Empty combo defaults to DefaultUICombo.
func ParseUICombo(combo string) (UISelection, error) {
	combo = strings.TrimSpace(combo)
	if combo == "" {
		combo = DefaultUICombo
	}

	parts := strings.SplitN(combo, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return UISelection{}, fmt.Errorf(
			"invalid --ui %q: use adapter/package-manager (e.g. react/pnpm, vue/bun) or %s",
			combo,
			DatastarUICombo,
		)
	}

	left, right := parts[0], parts[1]
	if left == "templ" && right == "datastar" {
		return UISelection{}, nil
	}

	if !IsSupportedInertiaAdapter(left) {
		return UISelection{}, fmt.Errorf(
			"invalid --ui adapter %q: valid options are 'react', 'vue', 'svelte', or %s",
			left,
			DatastarUICombo,
		)
	}
	if !IsSupportedJavaScriptRuntime(right) {
		return UISelection{}, fmt.Errorf(
			"invalid --ui package manager %q: valid options are 'pnpm', 'bun', 'npm'",
			right,
		)
	}

	return UISelection{
		Inertia:        left,
		PackageManager: right,
	}, nil
}
