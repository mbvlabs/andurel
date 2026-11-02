// Package runtimeintegration is a standalone cross-module integration fixture
// for the coordinated router, kiks and Inertia error lifecycle.
//
// It lives at testdata/fixtures/runtime-integration and exercises the published
// anduin router, the published kiks cookie middleware, the published Inertia
// capture middleware and the published routing and telemetry modules together.
// The module consumes public versions through proxy.golang.org with
// GOSUMDB=sum.golang.org, has no local replace directives and is meant to be
// run with GOWORK=off from this directory.
//
// This is standalone published-module integration coverage. It is not full
// framework or site release CI, not a release approval and not an application.
// It must not be imported, published, tagged or used as a module pin.
package runtimeintegration
