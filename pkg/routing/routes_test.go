package routing

import "testing"

func TestIsInertiaDefaultsFalse(t *testing.T) {
	route := NewSimpleRoute("/", "pages.home", "")
	if route.IsInertia() {
		t.Fatal("expected IsInertia to default to false")
	}

	uuidRoute := NewRouteWithUUIDID("/:id", "widgets.show", "/widgets")
	if uuidRoute.IsInertia() {
		t.Fatal("expected IsInertia to default to false for UUID routes")
	}
}

func TestInertiaRouteOption(t *testing.T) {
	route := NewSimpleRoute("/", "pages.home", "", InertiaRoute())
	if !route.IsInertia() {
		t.Fatal("expected InertiaRoute option to enable the flag")
	}

	paramsRoute := NewRouteWithParams[any]("/:slug", "posts.show", "/posts", InertiaRoute())
	if !paramsRoute.IsInertia() {
		t.Fatal("expected InertiaRoute option to enable the flag for params routes")
	}
}
