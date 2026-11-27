package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyFrontendAssetRoutes(t *testing.T) {
	route := `package routes
import r "example.org/routing"
const prefix = "/assets"
var ViteBuild = r.NewSimpleRoute("/dist/*", "vite.build", prefix)
`
	renderer := `package app
import ( i "example.org/inertia"; r "example.org/routes"; h "example.org/handlers"; web "example.org/anduin"; a "example.org/assets" )
func New() any {
	assetsHandler, err := h.NewAssetsHandler()
	_ = err
	router := web.NewRouter()
	assetsHandler.Register(router)
	return router
}
func NewRenderer(hosts any) any {
	return i.NewRenderer(i.RendererConfig{BuildPathURL: r.ViteBuild.FullURL(hosts)}, i.WithAssetFS(a.Files))
}
`
	handler := `package handlers
import ( filesystem "io/fs"; a "example.org/assets"; r "example.org/routes"; web "example.org/anduin" )
type AssetsHandler struct { dist any }
func NewAssetsHandler() (AssetsHandler, error) {
	dist, err := filesystem.Sub(a.Files, "dist")
	_ = err
	h, err := web.StaticFS(dist)
	_ = err
	return AssetsHandler{dist: h}, nil
}
func (a AssetsHandler) Register(rt any) {
	rt.Route(r.ViteBuild).Get(a.dist)
}
`
	tests := []struct {
		name, route, renderer, handler string
		wantViolation, wantBlocked     bool
	}{
		{"healthy", route, renderer, handler, false, false},
		{"equivalent route", strings.Replace(strings.Replace(route, `"/assets"`, `"/assets/dist"`, 1), `"/dist/*"`, `"/*"`, 1), renderer, handler, false, false},
		{"route normalization", strings.Replace(strings.Replace(route, `"/assets"`, `"/asset"`, 1), `"/dist/*"`, `"s/dist/*"`, 1), renderer, handler, false, true},
		{"wrong route", strings.Replace(route, "/dist/*", "/other/*", 1), renderer, handler, true, false},
		{"reserved route", route + `var Manual = r.NewSimpleRoute("/dist/manual.pdf", "manual", prefix)` + "\n", renderer, handler, true, false},
		{"reserved normalized route", route + `var Manual = r.NewSimpleRoute("assets//dist/manual.pdf", "manual", "")` + "\n", renderer, handler, true, false},
		{"dynamic conflicting route", route + `var Manual = r.NewSimpleRoute(dynamic(), "manual", prefix)` + "\n", renderer, handler, false, true},
		{"commented route", strings.Replace(route, "var ViteBuild", "// var ViteBuild", 1), renderer, handler, true, false},
		{"wrong subtree", route, renderer, strings.Replace(handler, `"dist"`, `"other"`, 1), true, false},
		{"hardcoded renderer", route, strings.Replace(renderer, "r.ViteBuild.FullURL(hosts)", `"/assets/dist/*"`, 1), handler, true, false},
		{"missing asset filesystem", route, strings.Replace(renderer, ", i.WithAssetFS(a.Files)", "", 1), handler, true, false},
		{"dynamic asset filesystem", route, strings.Replace(renderer, "i.WithAssetFS(a.Files)", "assetOption()", 1), handler, false, true},
		{"dynamic renderer", route, strings.Replace(renderer, "r.ViteBuild.FullURL(hosts)", "dynamic()", 1), handler, false, true},
		{"inactive registration", route, renderer, strings.Replace(handler, "rt.Route(r.ViteBuild).Get(a.dist)", "// rt.Route(r.ViteBuild).Get(a.dist)", 1), true, false},
		{"unused static", route, renderer, strings.Replace(handler, "dist: h", "dist: other", 1), false, true},
		{"app registration absent", route, strings.Replace(renderer, "assetsHandler.Register(router)", "// assetsHandler.Register(router)", 1), handler, true, false},
		{"app helper registration", route, strings.Replace(renderer, "assetsHandler.Register(router)", "registerAssets(assetsHandler, router)", 1), handler, false, true},
		{"app helper assignment", route, strings.Replace(renderer, "assetsHandler.Register(router)", "x := registerAssets(assetsHandler, router)\n _ = x", 1), handler, false, true},
		{"app conditional registration", route, strings.Replace(renderer, "assetsHandler.Register(router)", "if condition { assetsHandler.Register(router) }", 1), handler, false, true},
		{"app handler mutation", route, strings.Replace(renderer, "assetsHandler.Register(router)", "if condition { assetsHandler = other }\n assetsHandler.Register(router)", 1), handler, false, true},
		{"renderer config override", route, strings.Replace(renderer, "return i.NewRenderer(i.RendererConfig{BuildPathURL: r.ViteBuild.FullURL(hosts)}, i.WithAssetFS(a.Files))", `cfg := i.RendererConfig{BuildPathURL: r.ViteBuild.FullURL(hosts)}
cfg.BuildPathURL = "/wrong"
return i.NewRenderer(cfg, i.WithAssetFS(a.Files))`, 1), handler, false, true},
		{"renderer alias override", route, strings.Replace(renderer, "return i.NewRenderer(i.RendererConfig{BuildPathURL: r.ViteBuild.FullURL(hosts)}, i.WithAssetFS(a.Files))", `cfg := i.RendererConfig{BuildPathURL: r.ViteBuild.FullURL(hosts)}
alias := cfg
alias.BuildPathURL = "/wrong"
return i.NewRenderer(alias, i.WithAssetFS(a.Files))`, 1), handler, false, true},
		{"conditional static override", route, renderer, strings.Replace(handler, "return AssetsHandler{dist: h}", "if condition { h = other }\n return AssetsHandler{dist: h}", 1), false, true},
		{"register receiver override", route, renderer, strings.Replace(handler, "rt.Route(r.ViteBuild)", "if condition { a.dist = other }\n rt.Route(r.ViteBuild)", 1), false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			for name, contents := range map[string]string{"routes/assets.go": tt.route, "app/runtime.go": tt.renderer, "handlers/assets.go": tt.handler} {
				file := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, []byte(contents), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			violations, blocked := verifyFrontendAssetRoutes(root)
			if (len(violations) != 0) != tt.wantViolation || (blocked != "") != tt.wantBlocked {
				t.Fatalf("violations = %+v, blocked = %q", violations, blocked)
			}
			for _, v := range violations {
				if v.Rule != "frontend-setup" || v.File == "" || v.Fix == "" {
					t.Fatalf("not actionable: %+v", v)
				}
			}
		})
	}
}

func TestVerifyFrontendAssetRoutesGeneratedScaffold(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"routes/assets.go", "routes/api.go", "routes/pages.go", "routes/users.go", "app/runtime.go", "handlers/assets.go", "config/inertia.go"} {
		data, err := os.ReadFile(filepath.Join("..", "testdata", "golden", "new", "postgresql-inertia-react", name+".golden"))
		if err != nil {
			t.Fatal(err)
		}
		dest := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	violations, blocked := verifyFrontendAssetRoutes(root)
	if len(violations) != 0 || blocked != "" {
		t.Fatalf("generated scaffold: violations = %+v, blocked = %q", violations, blocked)
	}
	if entry, key, reason := frontendRendererEntry(root); entry != "resources/js/app.tsx" || key != "INERTIA_ENTRY_POINT" || reason != "" {
		t.Fatalf("runtime-configured entry: entry = %q, key = %q, reason = %q", entry, key, reason)
	}
}

func TestFrontendRendererEntry(t *testing.T) {
	for _, tt := range []struct {
		name, config, setup, want string
		blocked                   bool
	}{
		{"literal", `"resources/js/app.ts"`, "", "resources/js/app.ts", false},
		{"local constant", `entry`, `const entry = "resources/js/" + "app.ts"`, "resources/js/app.ts", false},
		{"dynamic", `cfg.Inertia.EntryPoint`, "", "", true},
		{"mutation", `entry`, `entry := "resources/js/app.ts"; if condition { entry = "other" }`, "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, "app"), 0o755); err != nil {
				t.Fatal(err)
			}
			source := `package app
import i "example.org/inertia"
func NewRenderer() any {
` + tt.setup + `
return i.NewRenderer(i.RendererConfig{EntryPoint: ` + tt.config + `})
}`
			if err := os.WriteFile(filepath.Join(root, "app", "runtime.go"), []byte(source), 0o644); err != nil {
				t.Fatal(err)
			}
			got, key, reason := frontendRendererEntry(root)
			if got != tt.want || key != "" || (reason != "") != tt.blocked {
				t.Fatalf("entry = %q, key = %q, blocked = %q", got, key, reason)
			}
		})
	}
}

func TestFrontendConfiguredEntry(t *testing.T) {
	source := `package config
import e "example.org/env"
const fallback = "resources/js/app.ts"
func NewInertia() (Inertia, error) {
	loader := e.New()
	cfg := Inertia{EntryPoint: loader.String("INERTIA_ENTRY_POINT", fallback)}
	return cfg, nil
}`
	for _, tt := range []struct {
		name, source, entry, key string
		blocked                  bool
	}{
		{"canonical", source, "resources/js/app.ts", "INERTIA_ENTRY_POINT", false},
		{"different key", strings.Replace(source, "INERTIA_ENTRY_POINT", "CUSTOM_ENTRY", 1), "resources/js/app.ts", "CUSTOM_ENTRY", false},
		{"dynamic default", strings.Replace(source, "fallback)}", "dynamic())}", 1), "", "", true},
		{"mutated config", strings.Replace(source, "return cfg, nil", `cfg.EntryPoint = "other"; return cfg, nil`, 1), "", "", true},
		{"unused config", strings.Replace(source, "return cfg, nil", "return other, nil", 1), "", "", true},
		{"unknown env helper", strings.Replace(source, "e.New()", "unknown()", 1), "", "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, "config"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "config", "inertia.go"), []byte(tt.source), 0o644); err != nil {
				t.Fatal(err)
			}
			entry, key, reason := frontendConfiguredEntry(root)
			if entry != tt.entry || key != tt.key || (reason != "") != tt.blocked {
				t.Fatalf("entry = %q, key = %q, blocked = %q", entry, key, reason)
			}
		})
	}
}
