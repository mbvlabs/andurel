package layout

import (
	"fmt"
	"strings"
)

const (
	DefaultUICombo   = "react/pnpm"
	DefaultUIAdapter = "react"
	DefaultUIPackage = "pnpm"
)

// UISelection is the parsed result of an andurel new --ui value. Inertia is the
// only supported UI, so a valid selection always names an adapter and a package
// manager.
type UISelection struct {
	// Inertia is the adapter (react, vue, svelte).
	Inertia string
	// PackageManager is the JS package manager.
	PackageManager string
}

// IsInertia reports whether the selection uses Inertia. It is true for every
// valid selection.
func (s UISelection) IsInertia() bool {
	return IsSupportedInertiaAdapter(s.Inertia)
}

// ParseUICombo parses a compound --ui value such as "react/pnpm".
// Empty combo defaults to DefaultUICombo.
func ParseUICombo(combo string) (UISelection, error) {
	combo = strings.TrimSpace(combo)
	if combo == "" {
		combo = DefaultUICombo
	}

	parts := strings.SplitN(combo, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return UISelection{}, fmt.Errorf(
			"invalid --ui %q: use adapter/package-manager (e.g. react/pnpm, vue/bun, svelte/npm)",
			combo,
		)
	}

	left, right := parts[0], parts[1]
	if !IsSupportedInertiaAdapter(left) {
		return UISelection{}, fmt.Errorf(
			"invalid --ui adapter %q: valid options are 'react', 'vue', 'svelte'",
			left,
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
