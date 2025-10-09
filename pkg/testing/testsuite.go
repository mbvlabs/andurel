package testing

import (
	"testing"
)

// TestData represents a generic test case structure
type TestData struct {
	Name     string
	Input    interface{}
	Expected interface{}
	Error    error
}

// TestSuite organizes tests by functionality
type TestSuite struct {
	Unit        map[string]UnitTest
	Integration map[string]IntegrationTest
	E2E         map[string]EndToEndTest
}

// UnitTest represents a unit test
type UnitTest struct {
	Name        string
	Description string
	TestFunc    func(t *testing.T)
}

// IntegrationTest represents an integration test
type IntegrationTest struct {
	Name        string
	Description string
	Setup       func() error
	Teardown    func() error
	TestFunc    func(t *testing.T)
}

// EndToEndTest represents an end-to-end test
type EndToEndTest struct {
	Name        string
	Description string
	Setup       func() error
	Teardown    func() error
	TestFunc    func(t *testing.T)
}

// NewTestSuite creates a new test suite
func NewTestSuite() *TestSuite {
	return &TestSuite{
		Unit:        make(map[string]UnitTest),
		Integration: make(map[string]IntegrationTest),
		E2E:         make(map[string]EndToEndTest),
	}
}

// AddUnitTest adds a unit test to the suite
func (ts *TestSuite) AddUnitTest(name, description string, testFunc func(t *testing.T)) {
	ts.Unit[name] = UnitTest{
		Name:        name,
		Description: description,
		TestFunc:    testFunc,
	}
}

// AddIntegrationTest adds an integration test to the suite
func (ts *TestSuite) AddIntegrationTest(name, description string, setup, teardown func() error, testFunc func(t *testing.T)) {
	ts.Integration[name] = IntegrationTest{
		Name:        name,
		Description: description,
		Setup:       setup,
		Teardown:    teardown,
		TestFunc:    testFunc,
	}
}

// AddE2ETest adds an end-to-end test to the suite
func (ts *TestSuite) AddE2ETest(name, description string, setup, teardown func() error, testFunc func(t *testing.T)) {
	ts.E2E[name] = EndToEndTest{
		Name:        name,
		Description: description,
		Setup:       setup,
		Teardown:    teardown,
		TestFunc:    testFunc,
	}
}

// RunUnitTests runs all unit tests in the suite
func (ts *TestSuite) RunUnitTests(t *testing.T) {
	for name, test := range ts.Unit {
		t.Run(name, test.TestFunc)
	}
}

// RunIntegrationTests runs all integration tests in the suite
func (ts *TestSuite) RunIntegrationTests(t *testing.T) {
	for name, test := range ts.Integration {
		t.Run(name, func(t *testing.T) {
			if test.Setup != nil {
				if err := test.Setup(); err != nil {
					t.Fatalf("Setup failed: %v", err)
				}
				defer func() {
					if test.Teardown != nil {
						if err := test.Teardown(); err != nil {
							t.Errorf("Teardown failed: %v", err)
						}
					}
				}()
			}
			test.TestFunc(t)
		})
	}
}

// RunE2ETests runs all end-to-end tests in the suite
func (ts *TestSuite) RunE2ETests(t *testing.T) {
	for name, test := range ts.E2E {
		t.Run(name, func(t *testing.T) {
			if test.Setup != nil {
				if err := test.Setup(); err != nil {
					t.Fatalf("Setup failed: %v", err)
				}
				defer func() {
					if test.Teardown != nil {
						if err := test.Teardown(); err != nil {
							t.Errorf("Teardown failed: %v", err)
						}
					}
				}()
			}
			test.TestFunc(t)
		})
	}
}

// RunAllTests runs all tests in the suite
func (ts *TestSuite) RunAllTests(t *testing.T) {
	t.Run("Unit", ts.RunUnitTests)
	t.Run("Integration", ts.RunIntegrationTests)
	t.Run("E2E", ts.RunE2ETests)
}

// TableDrivenTest provides a consistent pattern for table-driven tests
type TableDrivenTest struct {
	Name     string
	Tests    []TestData
	TestFunc func(t *testing.T, test TestData)
}

// Run executes the table-driven test
func (tdt *TableDrivenTest) Run(t *testing.T) {
	for _, test := range tdt.Tests {
		t.Run(test.Name, func(t *testing.T) {
			tdt.TestFunc(t, test)
		})
	}
}

// NewTableDrivenTest creates a new table-driven test
func NewTableDrivenTest(name string, tests []TestData, testFunc func(t *testing.T, test TestData)) *TableDrivenTest {
	return &TableDrivenTest{
		Name:     name,
		Tests:    tests,
		TestFunc: testFunc,
	}
}
