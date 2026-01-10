package e2e

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/e2e/internal"
	"github.com/mbvlabs/andurel/pkg/constants"
)

var updateResourceGolden = flag.Bool(
	"update-resource-golden",
	false,
	"update resource golden files",
)

// TestResourcePluralization validates that the `andurel generate resource` command
// correctly generates files with proper singular/plural forms.
//
// This test verifies the fix in commit 9d0088a which replaced hardcoded pluralization
// logic with the inflection library to handle irregular English plurals correctly.
//
// For a resource like "Company":
//   - Model struct should be singular: `type Company struct`
//   - Model functions should use proper plurals: `AllCompanies`, `PaginateCompanies`
//   - Table name should be plural: `companies`
//   - Query names should use proper plurals: `QueryCompanies`, `CountCompanies`
//   - Controller type should be singular: `type Company struct`
//   - Controller receiver should use proper naming (e.g., `c Company`)
//   - View headings should use proper plurals: `<h1>Companies</h1>`
func TestResourcePluralization(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource pluralization golden test in short mode")
	}

	binary := buildAndurelBinary(t)

	testCases := []struct {
		name             string
		resourceName     string
		tableName        string
		expectedPlural   string
		expectedSingular string
		columns          []string
	}{
		{
			name:             "company",
			resourceName:     "Company",
			tableName:        "companies",
			expectedPlural:   "Companies",
			expectedSingular: "Company",
			columns: []string{
				"name VARCHAR(200) NOT NULL",
				"industry VARCHAR(100)",
			},
		},
		{
			name:             "project",
			resourceName:     "Project",
			tableName:        "projects",
			expectedPlural:   "Projects",
			expectedSingular: "Project",
			columns: []string{
				"name VARCHAR(200) NOT NULL",
				"description TEXT",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			project := internal.NewProjectWithDatabase(t, binary, "postgresql")

			err := project.Scaffold("-c", "tailwind")
			internal.AssertCommandSucceeds(t, err, "scaffold")

			// Create the migration
			createMigration(t, project, "000100_create_"+tc.tableName, tc.tableName, tc.columns)

			// Generate the resource
			err = project.Generate("generate", "resource", tc.resourceName)
			internal.AssertCommandSucceeds(t, err, "generate resource")

			// Verify all expected files exist
			expectedFiles := []string{
				"models/" + strings.ToLower(tc.resourceName) + ".go",
				"models/factories/" + strings.ToLower(tc.resourceName) + ".go",
				"database/queries/" + tc.tableName + ".sql",
				"controllers/" + tc.tableName + ".go",
				"views/" + tc.tableName + "_resource.templ",
				"router/routes/" + tc.tableName + ".go",
				"router/connect_" + tc.tableName + "_routes.go",
			}

			for _, f := range expectedFiles {
				internal.AssertFileExists(t, project, f)
			}

			// Run golden file comparisons for each generated file
			t.Run("model_pluralization", func(t *testing.T) {
				validateModelPluralization(t, project, tc)
			})

			t.Run("queries_pluralization", func(t *testing.T) {
				validateQueriesPluralization(t, project, tc)
			})

			t.Run("controller_pluralization", func(t *testing.T) {
				validateControllerPluralization(t, project, tc)
			})

			t.Run("view_pluralization", func(t *testing.T) {
				validateViewPluralization(t, project, tc)
			})

			t.Run("routes_pluralization", func(t *testing.T) {
				validateRoutesPluralization(t, project, tc)
			})

			t.Run("router_registration_pluralization", func(t *testing.T) {
				validateRouterRegistrationPluralization(t, project, tc)
			})
		})
	}
}

type pluralizationTestCase struct {
	name             string
	resourceName     string
	tableName        string
	expectedPlural   string
	expectedSingular string
	columns          []string
}

func validateModelPluralization(t *testing.T, project *internal.Project, tc pluralizationTestCase) {
	t.Helper()

	modelPath := filepath.Join(project.Dir, "models", strings.ToLower(tc.resourceName)+".go")
	content, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatalf("Failed to read model file: %v", err)
	}

	contentStr := string(content)

	// Check for correct pluralization patterns - MUST be present
	correctPatterns := []struct {
		pattern string
		desc    string
	}{
		{"type " + tc.expectedSingular + " struct", "Model struct uses singular form"},
		{"func Find" + tc.expectedSingular + "(", "Find function uses singular form"},
		{"func Create" + tc.expectedSingular + "(", "Create function uses singular form"},
		{"func Update" + tc.expectedSingular + "(", "Update function uses singular form"},
		{"func Destroy" + tc.expectedSingular + "(", "Destroy function uses singular form"},
		{"func All" + tc.expectedPlural + "(", "All function uses correct plural form"},
		{"func Paginate" + tc.expectedPlural + "(", "Paginate function uses correct plural form"},
		{
			"type Paginated" + tc.expectedPlural + " struct",
			"Paginated struct uses correct plural form",
		},
		{"func Upsert" + tc.expectedSingular + "(", "Upsert function uses singular form"},
	}

	for _, p := range correctPatterns {
		if !strings.Contains(contentStr, p.pattern) {
			t.Errorf("Model file should contain %q (%s)", p.pattern, p.desc)
		}
	}

	// Check for INCORRECT pluralization - MUST NOT be present
	// These would be present if hardcoded "s" suffix was used
	incorrectPatterns := []struct {
		pattern string
		desc    string
	}{
		{"func AllCompanys(", "Should NOT use naive plural 'Companys'"},
		{"func PaginateCompanys(", "Should NOT use naive plural 'PaginateCompanys'"},
		{"type PaginatedCompanys", "Should NOT use naive plural 'PaginatedCompanys'"},
	}

	for _, p := range incorrectPatterns {
		if strings.Contains(contentStr, p.pattern) {
			t.Errorf("Model file should NOT contain %q (%s)", p.pattern, p.desc)
		}
	}

	// Golden file comparison
	goldenPath := filepath.Join("testdata", "golden", "resource", tc.name+"_model.golden")
	compareOrUpdateGolden(t, goldenPath, contentStr)
}

func validateQueriesPluralization(
	t *testing.T,
	project *internal.Project,
	tc pluralizationTestCase,
) {
	t.Helper()

	queriesPath := filepath.Join(project.Dir, "database", "queries", tc.tableName+".sql")
	content, err := os.ReadFile(queriesPath)
	if err != nil {
		t.Fatalf("Failed to read queries file: %v", err)
	}

	contentStr := string(content)

	// Check for correct patterns
	correctPatterns := []struct {
		pattern string
		desc    string
	}{
		{"from " + tc.tableName, "Uses plural table name"},
		{"into\n    " + tc.tableName, "INSERT uses plural table name"},
		{"update " + tc.tableName, "UPDATE uses plural table name"},
		{"-- name: Query" + tc.expectedSingular + "ByID", "QueryByID uses singular"},
		{"-- name: Query" + tc.expectedPlural, "Query uses correct plural"},
		{"-- name: Insert" + tc.expectedSingular, "Insert uses singular"},
		{"-- name: Update" + tc.expectedSingular, "Update uses singular"},
		{"-- name: Delete" + tc.expectedSingular, "Delete uses singular"},
		{"-- name: QueryPaginated" + tc.expectedPlural, "QueryPaginated uses correct plural"},
		{"-- name: Count" + tc.expectedPlural, "Count uses correct plural"},
		{"-- name: Upsert" + tc.expectedSingular, "Upsert uses singular"},
	}

	for _, p := range correctPatterns {
		if !strings.Contains(contentStr, p.pattern) {
			t.Errorf("Queries file should contain %q (%s)", p.pattern, p.desc)
		}
	}

	// Check for incorrect patterns
	incorrectPatterns := []struct {
		pattern string
		desc    string
	}{
		{"-- name: QueryCompanys", "Should NOT use naive plural"},
		{"-- name: CountCompanys", "Should NOT use naive plural"},
		{"-- name: QueryPaginatedCompanys", "Should NOT use naive plural"},
	}

	for _, p := range incorrectPatterns {
		if strings.Contains(contentStr, p.pattern) {
			t.Errorf("Queries file should NOT contain %q (%s)", p.pattern, p.desc)
		}
	}

	// Golden file comparison
	goldenPath := filepath.Join("testdata", "golden", "resource", tc.name+"_queries.golden")
	compareOrUpdateGolden(t, goldenPath, contentStr)
}

func validateControllerPluralization(
	t *testing.T,
	project *internal.Project,
	tc pluralizationTestCase,
) {
	t.Helper()

	controllerPath := filepath.Join(project.Dir, "controllers", tc.tableName+".go")
	content, err := os.ReadFile(controllerPath)
	if err != nil {
		t.Fatalf("Failed to read controller file: %v", err)
	}

	contentStr := string(content)

	// Check for correct patterns
	// Note: Controllers now use singular type names (e.g., Company, not Companies)
	// and the receiver name is derived from the type name (e.g., c Company, p Project)
	correctPatterns := []struct {
		pattern string
		desc    string
	}{
		{"type " + tc.expectedPlural + " struct", "Controller type uses plural"},
		{"func New" + tc.expectedPlural + "(", "Constructor uses plural"},
		{") Index(etx echo.Context)", "Index uses etx parameter"},
		{") Show(etx echo.Context)", "Show uses etx parameter"},
		{") Create(etx echo.Context)", "Create uses etx parameter"},
		{") Update(etx echo.Context)", "Update uses etx parameter"},
		{") Destroy(etx echo.Context)", "Destroy uses etx parameter"},
		{"models.Paginate" + tc.expectedPlural + "(", "Calls model with correct plural"},
		{"models.Find" + tc.expectedSingular + "(", "Calls Find with singular"},
		{"models.Create" + tc.expectedSingular + "(", "Calls Create with singular"},
		{"models.Update" + tc.expectedSingular + "(", "Calls Update with singular"},
		{"models.Destroy" + tc.expectedSingular + "(", "Calls Destroy with singular"},
	}

	for _, p := range correctPatterns {
		if !strings.Contains(contentStr, p.pattern) {
			t.Errorf("Controller file should contain %q (%s)", p.pattern, p.desc)
		}
	}

	// Check for incorrect patterns
	incorrectPatterns := []struct {
		pattern string
		desc    string
	}{
		{"type Companys struct", "Should NOT use naive plural 'Companys'"},
		{"func NewCompanys(", "Should NOT use naive plural"},
		{"models.PaginateCompanys(", "Should NOT use naive plural"},
		{"(c echo.Context)", "Should NOT use 'c' for echo.Context (use etx)"},
	}

	for _, p := range incorrectPatterns {
		if strings.Contains(contentStr, p.pattern) {
			t.Errorf("Controller file should NOT contain %q (%s)", p.pattern, p.desc)
		}
	}

	// Golden file comparison
	goldenPath := filepath.Join("testdata", "golden", "resource", tc.name+"_controller.golden")
	compareOrUpdateGolden(t, goldenPath, contentStr)
}

func validateViewPluralization(t *testing.T, project *internal.Project, tc pluralizationTestCase) {
	t.Helper()

	viewPath := filepath.Join(project.Dir, "views", tc.tableName+"_resource.templ")
	content, err := os.ReadFile(viewPath)
	if err != nil {
		t.Fatalf("Failed to read view file: %v", err)
	}

	contentStr := string(content)

	// Check for correct patterns - views should display the proper plural
	correctPatterns := []struct {
		pattern string
		desc    string
	}{
		{"<h1>" + tc.expectedPlural + "</h1>", "Heading uses correct plural"},
		{"templ " + tc.expectedSingular + "Index", "Index template uses singular prefix"},
		{"templ " + tc.expectedSingular + "Show", "Show template uses singular prefix"},
		{"templ " + tc.expectedSingular + "New", "New template uses singular prefix"},
		{"templ " + tc.expectedSingular + "Edit", "Edit template uses singular prefix"},
		{"models." + tc.expectedSingular, "References model with singular"},
	}

	for _, p := range correctPatterns {
		if !strings.Contains(contentStr, p.pattern) {
			t.Errorf("View file should contain %q (%s)", p.pattern, p.desc)
		}
	}

	// Check for incorrect patterns
	incorrectPatterns := []struct {
		pattern string
		desc    string
	}{
		{"<h1>Companys</h1>", "Should NOT use naive plural 'Companys' in heading"},
	}

	for _, p := range incorrectPatterns {
		if strings.Contains(contentStr, p.pattern) {
			t.Errorf("View file should NOT contain %q (%s)", p.pattern, p.desc)
		}
	}

	// Golden file comparison
	goldenPath := filepath.Join("testdata", "golden", "resource", tc.name+"_view.golden")
	compareOrUpdateGolden(t, goldenPath, contentStr)
}

func validateRoutesPluralization(
	t *testing.T,
	project *internal.Project,
	tc pluralizationTestCase,
) {
	t.Helper()

	routesPath := filepath.Join(project.Dir, "router", "routes", tc.tableName+".go")
	content, err := os.ReadFile(routesPath)
	if err != nil {
		t.Fatalf("Failed to read routes file: %v", err)
	}

	contentStr := string(content)

	// Check for correct patterns
	correctPatterns := []struct {
		pattern string
		desc    string
	}{
		{
			"const " + tc.expectedSingular + "Prefix = \"/" + tc.tableName + "\"",
			"Prefix constant uses singular with plural value",
		},
		{"var " + tc.expectedSingular + "Index", "Index route uses singular prefix"},
		{"var " + tc.expectedSingular + "Show", "Show route uses singular prefix"},
		{"var " + tc.expectedSingular + "New", "New route uses singular prefix"},
		{"var " + tc.expectedSingular + "Create", "Create route uses singular prefix"},
		{"var " + tc.expectedSingular + "Edit", "Edit route uses singular prefix"},
		{"var " + tc.expectedSingular + "Update", "Update route uses singular prefix"},
		{"var " + tc.expectedSingular + "Destroy", "Destroy route uses singular prefix"},
	}

	for _, p := range correctPatterns {
		if !strings.Contains(contentStr, p.pattern) {
			t.Errorf("Routes file should contain %q (%s)", p.pattern, p.desc)
		}
	}

	// Golden file comparison
	goldenPath := filepath.Join("testdata", "golden", "resource", tc.name+"_routes.golden")
	compareOrUpdateGolden(t, goldenPath, contentStr)
}

func validateRouterRegistrationPluralization(
	t *testing.T,
	project *internal.Project,
	tc pluralizationTestCase,
) {
	t.Helper()

	routerPath := filepath.Join(project.Dir, "router", "connect_"+tc.tableName+"_routes.go")
	content, err := os.ReadFile(routerPath)
	if err != nil {
		t.Fatalf("Failed to read router registration file: %v", err)
	}

	contentStr := string(content)

	lowercaseSingular := strings.ToLower(tc.expectedSingular)

	// Check for correct patterns
	// Note: Controllers now use singular type names (e.g., Company, not Companies)
	correctPatterns := []struct {
		pattern string
		desc    string
	}{
		{
			"func register" + tc.expectedSingular + "Routes(",
			"Registration function uses singular",
		},
		{
			lowercaseSingular + " controllers." + tc.expectedSingular,
			"Controller parameter uses singular",
		},
		{
			"routes." + tc.expectedSingular + "Index",
			"Uses singular for route constants",
		},
		{
			lowercaseSingular + ".Index",
			"Uses lowercase singular for controller method calls",
		},
	}

	for _, p := range correctPatterns {
		if !strings.Contains(contentStr, p.pattern) {
			t.Errorf("Router registration file should contain %q (%s)", p.pattern, p.desc)
		}
	}

	// Golden file comparison
	goldenPath := filepath.Join("testdata", "golden", "resource", tc.name+"_router.golden")
	compareOrUpdateGolden(t, goldenPath, contentStr)
}

func compareOrUpdateGolden(t *testing.T, goldenPath, actual string) {
	t.Helper()

	fullGoldenPath := filepath.Join(".", goldenPath)

	if *updateResourceGolden {
		err := os.MkdirAll(filepath.Dir(fullGoldenPath), constants.DirPermissionDefault)
		if err != nil {
			t.Fatalf("Failed to create golden directory: %v", err)
		}
		err = os.WriteFile(fullGoldenPath, []byte(actual), constants.FilePermissionPrivate)
		if err != nil {
			t.Fatalf("Failed to write golden file: %v", err)
		}
		t.Logf("Updated golden file: %s", goldenPath)
		return
	}

	expected, err := os.ReadFile(fullGoldenPath)
	if err != nil {
		t.Fatalf(
			"Failed to read golden file %s: %v\nRun 'go test -update-resource-golden' to create it",
			goldenPath,
			err,
		)
	}

	if string(expected) != actual {
		t.Errorf(
			"Generated code differs from golden file %s.\n\nRun 'go test -update-resource-golden' to update golden files if changes are expected.",
			goldenPath,
		)
	}
}
