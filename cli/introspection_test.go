package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/mbvlabs/andurel/v2/layout"
	"github.com/spf13/cobra"
)

func TestReadGoModMetadata(t *testing.T) {
	root := t.TempDir()
	writeTestFile(
		t,
		root,
		"go.mod",
		"module example.com/acme/orders\n\ngo 1.27.1\nrequire example.com/dep v1.0.0\n",
	)

	module, goVersion, err := readGoModMetadata(root)
	if err != nil {
		t.Fatalf("readGoModMetadata: %v", err)
	}
	if module != "example.com/acme/orders" || goVersion != "1.27.1" {
		t.Fatalf("metadata = module %q go %q", module, goVersion)
	}
}

func TestListProjectFilesAndSorting(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "domains/zeta.go", "package domains\n")
	writeTestFile(t, root, "domains/user/alpha.go", "package user\n")
	writeTestFile(t, root, "domains/readme.md", "ignore\n")

	items, err := listProjectFiles(root, "domains", ".go", "domain")
	if err != nil {
		t.Fatalf("listProjectFiles: %v", err)
	}
	want := []projectItem{
		{Name: "alpha", Path: "domains/user/alpha.go", Kind: "domain"},
		{Name: "zeta", Path: "domains/zeta.go", Kind: "domain"},
	}
	if !reflect.DeepEqual(items, want) {
		t.Fatalf("items = %#v, want %#v", items, want)
	}

	missing, err := listProjectFiles(root, "handlers", ".go", "handler")
	if err != nil {
		t.Fatalf("listProjectFiles missing: %v", err)
	}
	if len(missing) != 0 {
		t.Fatalf("missing directory should return no items, got %#v", missing)
	}
}

func TestIsDomainInternalFile(t *testing.T) {
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"domains/internal/queries/db.go", true},
		{"domains/internal/queries/nested/extra.go", true},
		{"domains/internal/notes.go", true},
		{"domains/user.go", false},
		{"domains/internal_notes.go", false},
		{"internal/runtime/runtime.go", false},
	} {
		if got := isDomainInternalFile(tc.path); got != tc.want {
			t.Errorf("isDomainInternalFile(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestDomainsCommandExcludesInternalGeneratedFiles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/app\n\ngo 1.27.1\n")
	writeTestFile(t, root, "domains/user.go", "package domains\n"+
		"\n"+
		"type User struct {\n"+
		"\tID string `andurel:\"id\"`\n"+
		"\tEmail string `andurel:\"email\"`\n"+
		"}\n"+
		"\n"+
		"type Users struct{}\n"+
		"\n"+
		"func (*User) Validate() error { return nil }\n")
	writeTestFile(t, root, "domains/user.sql", "-- domain queries stay colocated\n")
	writeTestFile(t, root, "domains/internal/queries/db.go", "package queries\n")
	writeTestFile(t, root, "domains/internal/queries/models.go", "package queries\n")
	writeTestFile(t, root, "domains/internal/queries/user.sql.go", "package queries\n")
	writeTestFile(t, root, "domains/internal/queries/nested/extra.go", "package nested\n")
	writeTestFile(t, root, "internal/runtime/runtime.go", "package runtime\n"+
		"\n"+
		"import \"example.com/app/domains\"\n"+
		"\n"+
		"func New() { _ = domains.NewUsers(nil) }\n")

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	var out bytes.Buffer
	cmd := newDomainsCommand()
	output.RegisterPersistentFlags(cmd)
	cmd.SetOut(&out)
	_ = cmd.PersistentFlags().Set("json", "true")
	if err := cmd.Execute(); err != nil {
		t.Fatalf("inspect domains: %v", err)
	}

	var envelope struct {
		OK   bool          `json:"ok"`
		Data []projectItem `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope: %v\n%s", err, out.String())
	}
	if !envelope.OK {
		t.Fatalf("inspect domains failed:\n%s", out.String())
	}

	gotPaths := make([]string, 0, len(envelope.Data))
	for _, item := range envelope.Data {
		gotPaths = append(gotPaths, item.Path)
	}
	if want := []string{"domains/user.go"}; !reflect.DeepEqual(gotPaths, want) {
		t.Fatalf("items = %#v, want %#v", gotPaths, want)
	}

	entities := envelope.Data[0].Entities
	if len(entities) != 1 {
		t.Fatalf("entities = %#v, want one entity", entities)
	}
	entity := entities[0]
	if entity.Name != "User" || entity.Table != "users" || entity.TableSource != "inferred" {
		t.Fatalf("unexpected entity mapping: %#v", entity)
	}
	if !entity.Validates || entity.Receiver != "Users" || !entity.Wired {
		t.Fatalf("lost entity metadata: %#v", entity)
	}

	var human bytes.Buffer
	humanCmd := newDomainsCommand()
	humanCmd.SetOut(&human)
	if err := humanCmd.Execute(); err != nil {
		t.Fatalf("inspect domains human: %v", err)
	}
	if !strings.Contains(human.String(), "List domain files") {
		t.Fatalf("human output missing summary:\n%s", human.String())
	}
	if strings.Contains(human.String(), "domains/internal/") {
		t.Fatalf("human output leaked internal files:\n%s", human.String())
	}
}

func TestToolInfos(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "bin/templ", "#!/bin/sh\n")
	lock := layout.NewAndurelLock("v1.2.3")
	lock.Tools["templ"] = layout.NewBinaryTool("templ", "v0.3.1")
	lock.Tools["shadowfax"] = layout.NewBuiltTool(
		filepath.ToSlash(filepath.Join("cmd", "shadowfax")),
		"v1.0.0",
	)

	tools := toolInfos(root, lock)
	if len(tools) != 2 || tools[0].Name != "shadowfax" || tools[1].Name != "templ" {
		t.Fatalf("tools not sorted: %#v", tools)
	}
	if tools[0].BinaryPath != "cmd/shadowfax" || tools[0].Installed {
		t.Fatalf("unexpected built tool info: %#v", tools[0])
	}
	if tools[1].BinaryPath != "bin/templ" || !tools[1].Installed || tools[1].Version != "v0.3.1" {
		t.Fatalf("unexpected binary tool info: %#v", tools[1])
	}
}

func TestCollectProjectInfo(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/acme/orders\n\ngo 1.27.1\n")
	writeTestFile(t, root, "bin/goose", "#!/bin/sh\n")
	lock := layout.NewAndurelLock("v1.2.3")
	lock.ScaffoldConfig = &layout.ScaffoldConfig{
		ProjectName:       "orders",
		Database:          "postgres",
		Inertia:           "react",
		JavaScriptRuntime: "pnpm",
	}
	lock.DatabaseConfig = &layout.DatabaseConfig{
		Engine:   layout.DatabaseEnginePostgreSQL,
		NullType: layout.NullTypePGType,
	}
	lock.Tools["goose"] = validTestTool("goose", "v3.0.0")
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	info, err := collectProjectInfo(root)
	if err != nil {
		t.Fatalf("collectProjectInfo: %v", err)
	}
	if info.Root != root || info.Module != "example.com/acme/orders" ||
		info.GoVersion != "1.27.1" ||
		info.AndurelVersion != "v1.2.3" {
		t.Fatalf("unexpected project identity: %#v", info)
	}
	if info.ScaffoldConfig == nil || info.ScaffoldConfig.PackageManager() != "pnpm" {
		t.Fatalf("missing scaffold config: %#v", info.ScaffoldConfig)
	}
	if info.DatabaseConfig == nil ||
		info.DatabaseConfig.Engine != layout.DatabaseEnginePostgreSQL ||
		info.DatabaseConfig.NullType != layout.NullTypePGType {
		t.Fatalf("missing database config: %#v", info.DatabaseConfig)
	}
	if len(info.Tools) != 1 || !info.Tools[0].Installed {
		t.Fatalf("unexpected tools: %#v", info.Tools)
	}
}

func TestIntrospectionCommands(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/acme/orders\n\ngo 1.27.1\n")
	writeTestFile(t, root, "domains/errors.go", "package domains\n")
	writeTestFile(t, root, "domains/user/user.go", "package user\n")
	writeTestFile(t, root, "handlers/users.go", "package handlers\n")
	writeTestFile(t, root, "migrations/0001_init.sql", "-- noop\n")
	writeTestFile(t, root, "queue/jobs/send_email.go", "package jobs\n")
	writeTestFile(t, root, "queue/workers.go", "package queue\n")
	writeRouteManifestTestFile(t, root, "users.go", `package routes

import "example.com/app/pkg/routing"

const UserPrefix = "/users"

var UserIndex = routing.NewSimpleRoute("", "users.index", UserPrefix)
`)
	lock := layout.NewAndurelLock("v1.2.3")
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() {
		findGoModRoot = originalFindGoModRoot
	})

	for _, cmd := range []*cobra.Command{
		newProjectInfoCommand(),
		newDomainsCommand(),
		newMigrationsCommand(),
		newHandlersCommand(),
		newJobsCommand(),
	} {
		var out bytes.Buffer
		output.RegisterPersistentFlags(cmd)
		cmd.SetOut(&out)
		_ = cmd.PersistentFlags().Set("json", "true")
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%s command: %v", cmd.Use, err)
		}
		if !strings.Contains(out.String(), "ok") {
			t.Fatalf("%s command output missing envelope:\n%s", cmd.Use, out.String())
		}
	}

	var human bytes.Buffer
	routes := newRoutesCommand()
	routes.SetOut(&human)
	if err := routes.Execute(); err != nil {
		t.Fatalf("routes human: %v", err)
	}
	if !strings.Contains(human.String(), "UserIndex") {
		t.Fatalf("routes human output missing route:\n%s", human.String())
	}

	var quiet bytes.Buffer
	routes = newRoutesCommand()
	output.RegisterPersistentFlags(routes)
	routes.SetOut(&quiet)
	_ = routes.PersistentFlags().Set("quiet", "true")
	if err := routes.Execute(); err != nil {
		t.Fatalf("routes quiet: %v", err)
	}
	if quiet.Len() != 0 {
		t.Fatalf("quiet routes should not output, got:\n%s", quiet.String())
	}
}
