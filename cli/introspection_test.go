package cli

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/cli/output"
	"github.com/mbvlabs/andurel/layout"
	"github.com/spf13/cobra"
)

func TestReadGoModMetadata(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/acme/orders\n\ngo 1.26\nrequire example.com/dep v1.0.0\n")

	module, goVersion, err := readGoModMetadata(root)
	if err != nil {
		t.Fatalf("readGoModMetadata: %v", err)
	}
	if module != "example.com/acme/orders" || goVersion != "1.26" {
		t.Fatalf("metadata = module %q go %q", module, goVersion)
	}
}

func TestListProjectFilesAndSorting(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "models/zeta.go", "package models\n")
	writeTestFile(t, root, "models/admin/alpha.go", "package admin\n")
	writeTestFile(t, root, "models/readme.md", "ignore\n")

	items, err := listProjectFiles(root, "models", ".go", "model")
	if err != nil {
		t.Fatalf("listProjectFiles: %v", err)
	}
	want := []projectItem{
		{Name: "alpha", Path: "models/admin/alpha.go", Kind: "model"},
		{Name: "zeta", Path: "models/zeta.go", Kind: "model"},
	}
	if !reflect.DeepEqual(items, want) {
		t.Fatalf("items = %#v, want %#v", items, want)
	}

	missing, err := listProjectFiles(root, "controllers", ".go", "controller")
	if err != nil {
		t.Fatalf("listProjectFiles missing: %v", err)
	}
	if len(missing) != 0 {
		t.Fatalf("missing directory should return no items, got %#v", missing)
	}
}

func TestExtensionAndToolInfos(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "bin/templ", "#!/bin/sh\n")
	lock := layout.NewAndurelLock("v1.2.3")
	lock.Extensions["mail"] = &layout.Extension{AppliedAt: "2026-07-08T10:00:00Z"}
	lock.Extensions["aws"] = nil
	lock.Tools["templ"] = layout.NewBinaryTool("templ", "v0.3.1")
	lock.Tools["shadowfax"] = layout.NewBuiltTool(filepath.ToSlash(filepath.Join("cmd", "shadowfax")), "v1.0.0")

	extensions := extensionInfos(lock)
	if !reflect.DeepEqual(extensions, []extensionInfo{
		{Name: "aws"},
		{Name: "mail", AppliedAt: "2026-07-08T10:00:00Z"},
	}) {
		t.Fatalf("extensionInfos = %#v", extensions)
	}

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
	writeTestFile(t, root, "go.mod", "module example.com/acme/orders\n\ngo 1.26\n")
	writeTestFile(t, root, "bin/goose", "#!/bin/sh\n")
	lock := layout.NewAndurelLock("v1.2.3")
	lock.ScaffoldConfig = &layout.ScaffoldConfig{
		ProjectName:       "orders",
		Database:          "postgres",
		Inertia:           "react",
		JavaScriptRuntime: "pnpm",
	}
	lock.DatabaseConfig = &layout.DatabaseConfig{NullType: "sql.Null"}
	lock.Tools["goose"] = layout.NewBinaryTool("goose", "v3.0.0")
	lock.Extensions["docker"] = &layout.Extension{AppliedAt: "2026-07-08T10:00:00Z"}
	if err := lock.WriteLockFile(root); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	info, err := collectProjectInfo(root)
	if err != nil {
		t.Fatalf("collectProjectInfo: %v", err)
	}
	if info.Root != root || info.Module != "example.com/acme/orders" || info.GoVersion != "1.26" || info.AndurelVersion != "v1.2.3" {
		t.Fatalf("unexpected project identity: %#v", info)
	}
	if info.ScaffoldConfig == nil || info.ScaffoldConfig.JavaScriptRuntime != "pnpm" {
		t.Fatalf("missing scaffold config: %#v", info.ScaffoldConfig)
	}
	if info.DatabaseConfig == nil || info.DatabaseConfig.NullType != "sql.Null" {
		t.Fatalf("missing database config: %#v", info.DatabaseConfig)
	}
	if len(info.Extensions) != 1 || info.Extensions[0].Name != "docker" {
		t.Fatalf("unexpected extensions: %#v", info.Extensions)
	}
	if len(info.Tools) != 1 || !info.Tools[0].Installed {
		t.Fatalf("unexpected tools: %#v", info.Tools)
	}
	if info.ConfigPath != filepath.Join(root, ".andurel", "config.json") || info.UserConfigPath == "" || info.UserCacheDirectory == "" {
		t.Fatalf("unexpected paths: %#v", info)
	}
}

func TestIntrospectionCommands(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/acme/orders\n\ngo 1.26\n")
	writeTestFile(t, root, "models/user.go", "package models\n")
	writeTestFile(t, root, "controllers/users.go", "package controllers\n")
	writeTestFile(t, root, "views/users.templ", "package views\n")
	writeTestFile(t, root, "database/migrations/0001_init.sql", "-- noop\n")
	writeTestFile(t, root, "queue/jobs/send_email.go", "package jobs\n")
	writeTestFile(t, root, "queue/workers.go", "package queue\n")
	writeRouteManifestTestFile(t, root, "users.go", `package routes

import "example.com/app/internal/routing"

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
		newModelsCommand(),
		newMigrationsCommand(),
		newControllersCommand(),
		newViewsCommand(),
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
