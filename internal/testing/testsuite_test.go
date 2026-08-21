package testing

import (
	"reflect"
	"slices"
	gotesting "testing"
)

func TestTestSuiteAddAndRun(t *gotesting.T) {
	suite := NewTestSuite()
	if suite.Unit == nil || suite.Integration == nil || suite.E2E == nil {
		t.Fatal("expected initialized test maps")
	}

	var events []string
	suite.AddUnitTest("unit", "unit description", func(t *gotesting.T) {
		events = append(events, "unit")
	})
	suite.AddIntegrationTest(
		"integration",
		"integration description",
		func() error {
			events = append(events, "integration setup")
			return nil
		},
		func() error {
			events = append(events, "integration teardown")
			return nil
		},
		func(t *gotesting.T) {
			events = append(events, "integration")
		},
	)
	suite.AddE2ETest(
		"e2e",
		"e2e description",
		func() error {
			events = append(events, "e2e setup")
			return nil
		},
		func() error {
			events = append(events, "e2e teardown")
			return nil
		},
		func(t *gotesting.T) {
			events = append(events, "e2e")
		},
	)

	if suite.Unit["unit"].Description != "unit description" {
		t.Fatalf("unexpected unit metadata: %+v", suite.Unit["unit"])
	}
	if suite.Integration["integration"].Description != "integration description" {
		t.Fatalf("unexpected integration metadata: %+v", suite.Integration["integration"])
	}
	if suite.E2E["e2e"].Description != "e2e description" {
		t.Fatalf("unexpected e2e metadata: %+v", suite.E2E["e2e"])
	}

	suite.RunAllTests(t)

	for _, event := range []string{
		"unit",
		"integration setup",
		"integration",
		"integration teardown",
		"e2e setup",
		"e2e",
		"e2e teardown",
	} {
		if !containsEvent(events, event) {
			t.Fatalf("expected event %q in %v", event, events)
		}
	}
}

func TestTableDrivenTestRun(t *gotesting.T) {
	tdt := NewTableDrivenTest(
		"string cases",
		[]TestData{
			{Name: "first", Input: "a", Expected: "A"},
			{Name: "second", Input: "b", Expected: "B"},
		},
		func(t *gotesting.T, test TestData) {
			if test.Input == "" || test.Expected == "" {
				t.Fatalf("expected populated test data: %+v", test)
			}
		},
	)

	if tdt.Name != "string cases" {
		t.Fatalf("unexpected table name: %q", tdt.Name)
	}

	var names []string
	tdt.TestFunc = func(t *gotesting.T, test TestData) {
		names = append(names, test.Name)
	}
	tdt.Run(t)

	expected := []string{"first", "second"}
	if !reflect.DeepEqual(names, expected) {
		t.Fatalf("expected %v, got %v", expected, names)
	}
}

func containsEvent(events []string, want string) bool {
	return slices.Contains(events, want)
}
