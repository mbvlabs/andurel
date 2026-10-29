package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func findTestPackage(t *testing.T, step verifyStep, name string) verifyTestPackage {
	t.Helper()
	for _, pkg := range step.Packages {
		if pkg.Package == name {
			return pkg
		}
	}
	t.Fatalf("go test step has no package %q: %#v", name, step.Packages)
	return verifyTestPackage{}
}

func TestParseGoTestJSONCountsTopLevelTestsAndSubtestsSeparately(t *testing.T) {
	const events = `{"Action":"start","Package":"example.com/app/accounting"}
{"Action":"run","Package":"example.com/app/accounting","Test":"TestParent"}
{"Action":"run","Package":"example.com/app/accounting","Test":"TestParent/child"}
{"Action":"pass","Package":"example.com/app/accounting","Test":"TestParent/child"}
{"Action":"pass","Package":"example.com/app/accounting","Test":"TestParent"}
{"Action":"run","Package":"example.com/app/accounting","Test":"TestSkipped"}
{"Action":"output","Package":"example.com/app/accounting","Test":"TestSkipped","Output":"    accounting_test.go:9: needs the fixture server\n"}
{"Action":"skip","Package":"example.com/app/accounting","Test":"TestSkipped"}
{"Action":"output","Package":"example.com/app/accounting","Output":"PASS\n"}
{"Action":"output","Package":"example.com/app/accounting","Output":"ok  \texample.com/app/accounting\t(cached)\n"}
{"Action":"pass","Package":"example.com/app/accounting","Elapsed":0}
`
	result := parseGoTestJSON([]byte(events))
	step := verifyStep{Name: "go test"}
	step.Packages = result.packageReports()
	step.SkippedTests = result.skippedTests
	totals := result.totals()
	step.Totals = &totals
	pkg := findTestPackage(t, step, "example.com/app/accounting")
	if pkg.Status != verifyStatusOK || pkg.Passed != 1 || pkg.Skipped != 1 || pkg.Failed != 0 {
		t.Fatalf("package = %#v, want 1 passed, 1 skipped, 0 failed top-level", pkg)
	}
	if pkg.Subtests == nil || pkg.Subtests.Passed != 1 {
		t.Fatalf("package subtests = %#v, want one passing subtest", pkg.Subtests)
	}
	if !pkg.Cached {
		t.Fatalf("package = %#v, want cached", pkg)
	}
	if totals.Passed != 1 || totals.Skipped != 1 || totals.Failed != 0 {
		t.Fatalf("totals = %#v, want top-level counts only", totals)
	}
	if totals.SubtestsPassed != 1 {
		t.Fatalf("totals = %#v, want one passing subtest", totals)
	}
	if totals.CachedPackages != 1 || totals.Packages != 1 {
		t.Fatalf("totals = %#v, want one cached package", totals)
	}
	if len(step.SkippedTests) != 1 || step.SkippedTests[0].Test != "TestSkipped" ||
		!strings.Contains(step.SkippedTests[0].Reason, "fixture server") {
		t.Fatalf("skipped tests = %#v, want the preserved reason", step.SkippedTests)
	}
}

func TestParseGoTestJSONPackageOutcomes(t *testing.T) {
	const events = `{"Action":"start","Package":"example.com/app/pass"}
{"Action":"run","Package":"example.com/app/pass","Test":"TestOK"}
{"Action":"pass","Package":"example.com/app/pass","Test":"TestOK"}
{"Action":"output","Package":"example.com/app/pass","Output":"ok  \texample.com/app/pass\t0.010s\n"}
{"Action":"pass","Package":"example.com/app/pass","Elapsed":0.01}
{"Action":"start","Package":"example.com/app/notest"}
{"Action":"output","Package":"example.com/app/notest","Output":"?   \texample.com/app/notest\t[no test files]\n"}
{"Action":"skip","Package":"example.com/app/notest","Elapsed":0}
{"Action":"start","Package":"example.com/app/testmain"}
{"Action":"output","Package":"example.com/app/testmain","Output":"skipping db tests: docker not found\n"}
{"Action":"output","Package":"example.com/app/testmain","Output":"PASS\n"}
{"Action":"pass","Package":"example.com/app/testmain","Elapsed":0.01}
{"Action":"start","Package":"example.com/app/buildfail"}
{"Action":"output","Package":"example.com/app/buildfail","Output":"FAIL\texample.com/app/buildfail [build failed]\n"}
{"Action":"fail","Package":"example.com/app/buildfail","Elapsed":0,"FailedBuild":"example.com/app/buildfail"}
{"Action":"start","Package":"example.com/app/setupfail"}
{"Action":"output","Package":"example.com/app/setupfail","Output":"FAIL\texample.com/app/setupfail [setup failed]\n"}
{"Action":"fail","Package":"example.com/app/setupfail","Elapsed":0,"FailedBuild":"missing.example/pkg"}
{"Action":"start","Package":"example.com/app/timeout"}
{"Action":"output","Package":"example.com/app/timeout","Output":"panic: test timed out after 10m0s\n"}
{"Action":"fail","Package":"example.com/app/timeout","Elapsed":600}
{"Action":"start","Package":"example.com/app/race"}
{"Action":"run","Package":"example.com/app/race","Test":"TestRace"}
{"Action":"output","Package":"example.com/app/race","Output":"WARNING: DATA RACE\n"}
{"Action":"fail","Package":"example.com/app/race","Test":"TestRace"}
{"Action":"fail","Package":"example.com/app/race","Elapsed":1}
`
	result := parseGoTestJSON([]byte(events))
	step := verifyStep{Name: "go test"}
	step.Packages = result.packageReports()
	totals := result.totals()
	step.Totals = &totals
	step.SkippedPackages = result.skipped

	if pkg := findTestPackage(t, step, "example.com/app/pass"); pkg.Status != verifyStatusOK || pkg.Passed != 1 {
		t.Fatalf("pass package = %#v", pkg)
	}
	if pkg := findTestPackage(t, step, "example.com/app/notest"); pkg.Status != verifyTestStatusNoTestFiles || !pkg.NoTestFiles {
		t.Fatalf("no-test-files package = %#v", pkg)
	}
	if pkg := findTestPackage(t, step, "example.com/app/testmain"); pkg.Status != verifyTestStatusNoTestsRan ||
		!pkg.NoTestsRan || !strings.Contains(pkg.Reason, "docker not found") {
		t.Fatalf("TestMain package = %#v, want no_tests_ran with the reason", pkg)
	}
	if pkg := findTestPackage(t, step, "example.com/app/buildfail"); pkg.Status != verifyStatusFail || !pkg.BuildFailed {
		t.Fatalf("build failure package = %#v", pkg)
	}
	if pkg := findTestPackage(t, step, "example.com/app/setupfail"); pkg.Status != verifyStatusFail || !pkg.SetupFailed {
		t.Fatalf("setup failure package = %#v", pkg)
	}
	if pkg := findTestPackage(t, step, "example.com/app/timeout"); pkg.Status != verifyStatusFail || !pkg.TimedOut {
		t.Fatalf("timeout package = %#v", pkg)
	}
	if pkg := findTestPackage(t, step, "example.com/app/race"); pkg.Status != verifyStatusFail ||
		!pkg.RaceDetected || pkg.Failed != 1 {
		t.Fatalf("race package = %#v", pkg)
	}
	if totals.NoTestFiles != 1 || totals.NoTestsRan != 1 {
		t.Fatalf("totals = %#v, want one no-test-files and one no-tests-ran package", totals)
	}
	if len(step.SkippedPackages) != 1 || step.SkippedPackages[0].Package != "example.com/app/testmain" {
		t.Fatalf("skipped packages = %#v, want the TestMain package", step.SkippedPackages)
	}
	if totals.Failed != 1 {
		t.Fatalf("totals = %#v, want the one failing top-level test", totals)
	}
}

func TestParseGoTestJSONKeepsMalformedLines(t *testing.T) {
	result := parseGoTestJSON([]byte(
		"not json\n" +
			`{"Action":"start","Package":"example.com/app"}` + "\n" +
			`{"Action":"pass","Package":"example.com/app"}` + "\n",
	))
	if len(result.other) != 1 || result.other[0] != "not json" {
		t.Fatalf("other = %#v, want the malformed line preserved", result.other)
	}
	step := verifyStep{Name: "go test"}
	step.Packages = result.packageReports()
	if pkg := findTestPackage(t, step, "example.com/app"); pkg.Status != verifyTestStatusNoTestsRan {
		t.Fatalf("package = %#v, want no_tests_ran: a package with no test events is not a pass", pkg)
	}
}

func TestVerifyWithTestsReportsPackageAccounting(t *testing.T) {
	root := newVerifyTestProject(t)
	writeVerifyTestPackages(t, root)
	writeTestFile(t, root, "accounting/accounting.go", "package accounting\n")
	writeTestFile(t, root, "accounting/accounting_test.go", `package accounting

import "testing"

func TestParent(t *testing.T) {
	t.Run("child", func(t *testing.T) {})
}
`)

	result := runVerifyInProject(t, root, "verify", "--skip-generated", "--with-tests", "--json")
	if result.err != nil {
		t.Fatalf("verify --with-tests: %v\n%s", result.err, result.stderr)
	}
	report := decodeVerifyReport(t, result)
	step := findVerifyStep(t, report, "go test")
	if step.Totals == nil || len(step.Packages) == 0 {
		t.Fatalf("go test step has no accounting: %#v", step)
	}
	if step.Totals.Passed < 2 || step.Totals.Failed != 0 {
		t.Fatalf("totals = %#v, want at least the plain and parent tests", step.Totals)
	}
	if step.Totals.SubtestsPassed < 1 {
		t.Fatalf("totals = %#v, want the subtest counted separately", step.Totals)
	}
	if step.Totals.NoTestsRan != 1 {
		t.Fatalf("totals = %#v, want the TestMain package counted as no_tests_ran", step.Totals)
	}
	if pkg := findTestPackage(t, step, "example.com/app/dbtests"); pkg.Status != verifyTestStatusNoTestsRan {
		t.Fatalf("dbtests package = %#v, want no_tests_ran", pkg)
	}
	if len(step.SkippedPackages) != 1 {
		t.Fatalf("skipped_packages = %#v, want the preserved TestMain skip", step.SkippedPackages)
	}

	encoded, err := json.Marshal(step)
	if err != nil {
		t.Fatalf("marshal go test step: %v", err)
	}
	for _, want := range []string{`"packages"`, `"totals"`, `"skipped_packages"`, `"no_tests_ran"`} {
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("go test JSON is missing %s: %s", want, encoded)
		}
	}

	human := runVerifyInProject(t, root, "verify", "--skip-generated", "--with-tests")
	if human.err != nil {
		t.Fatalf("verify --with-tests: %v", human.err)
	}
	if !strings.Contains(human.stdout, "go test    ok (") ||
		!strings.Contains(human.stdout, "passed") ||
		!strings.Contains(human.stdout, "skipped example.com/app/dbtests") {
		t.Fatalf("human output is missing the accounting summary:\n%s", human.stdout)
	}
}
