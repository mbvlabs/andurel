package cli

import "testing"

func TestRouteUsageAllowsComponentWithDestructuredParameters(t *testing.T) {
	root := t.TempDir()
	writeRouteUsageFixture(t, root, `import { routes } from '../routes'
export function AuthLayout({ children }: { children: unknown }) {
  const links = [{ href: routes.homePage() }]
  return links
}
`)
	violations, _, applicable := analyzeRouteUsage(root)
	if !applicable {
		t.Fatal("route usage analysis did not run")
	}
	if len(violations) != 0 {
		t.Fatalf("deferred component body reported as module initialization: %#v", violations)
	}
}
