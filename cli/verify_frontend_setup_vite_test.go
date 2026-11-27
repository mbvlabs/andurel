package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Set ANDUREL_TEST_VITE_NODE_MODULES to an installed Vite node_modules directory
// to exercise the checker against a real Vite without installing dependencies.
func TestFrontendSetupResolvedVite(t *testing.T) {
	modules := os.Getenv("ANDUREL_TEST_VITE_NODE_MODULES")
	if modules == "" {
		t.Skip("set ANDUREL_TEST_VITE_NODE_MODULES to test with installed Vite")
	}
	modules, err := filepath.Abs(modules)
	if err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile("verify_frontend_setup.mjs")
	if err != nil {
		t.Fatal(err)
	}
	const config = `
import { defineConfig } from 'vite'
console.log('config noise')
const shared = { build: { outDir: 'assets/dist', manifest: 'vite/manifest.json', rollupOptions: { input: 'resources/js/app.ts' } } }
const reload = { name: 'andurel-reload', configureServer(server) {
  const file = process.env.ANDUREL_VITE_RELOAD_FILE
  server.watcher.add(file)
  server.watcher.on('change', changed => { if (changed === file) server.ws.send({type:'full-reload'}) })
} }
export default defineConfig(({command}) => ({
  ...shared, plugins: [reload],
  base: command === 'serve' ? '/assets/dist/' : './',
  server: { port: 6123, strictPort: true, watch: { ignored: ['**/resources/*.html'] }, origin: 'http://localhost:6123', cors: { origin: ['http://api.example:8081', /^http:\/\/([a-z-]+\.)?localhost:8080$/] } }
}))
`
	tests := []struct {
		name, config, want string
	}{
		{"valid", config, ""},
		{"url path", config, "URL path"},
		{"port", strings.Replace(config, "port: 6123", "port: 6124", 1), "server port"},
		{"strict port", strings.Replace(config, "strictPort: true", "strictPort: false", 1), "configured port"},
		{"production base", strings.Replace(config, ": './'", ": '/assets/dist/'", 1), "production Vite base"},
		{"production node env", strings.Replace(config, "outDir: 'assets/dist'", "outDir: process.env.NODE_ENV === 'production' ? 'wrong' : 'assets/dist'", 1), "production output"},
		{"ssr build", strings.Replace(config, "outDir: 'assets/dist'", "ssr: true, outDir: 'assets/dist'", 1), "configured for SSR"},
		{"write disabled", strings.Replace(config, "outDir: 'assets/dist'", "write: false, outDir: 'assets/dist'", 1), "not to write assets"},
		{"development node env", strings.Replace(config, "'/assets/dist/' : './'", "(process.env.NODE_ENV === 'development' ? '/wrong/' : '/assets/dist/') : './'", 1), "development Vite base"},
		{"cors primary", strings.Replace(config, "localhost:8080", "localhost:8081", 1), "CORS"},
		{"cors named", strings.Replace(config, "http://api.example:8081", "http://api.example:8082", 1), "api.example:8081"},
		{"primary-only cors when no subdomain configured", strings.Replace(config, "([a-z-]+\\.)?", "", 1), ""},
		{"unhashed assets", strings.Replace(config, "input: 'resources/js/app.ts'", "input: 'resources/js/app.ts', output: { assetFileNames: '[name][extname]' }", 1), "content hash"},
		{"wrong output dir", strings.Replace(config, "input: 'resources/js/app.ts'", "input: 'resources/js/app.ts', output: [{ dir: 'assets/dist' }, { dir: 'elsewhere' }]", 1), "Rollup output directory"},
		{"output file", strings.Replace(config, "input: 'resources/js/app.ts'", "input: 'resources/js/app.ts', output: { file: 'assets/dist/app.js' }", 1), "output.file"},
		{"watch disabled", strings.Replace(config, "watch: { ignored: ['**/resources/*.html'] }", "watch: null", 1), "watcher is disabled"},
		{"watch ignores tmp", strings.Replace(config, "**/resources/*.html", "**/tmp/**", 1), "watcher ignores"},
		{"watch ignored predicate", strings.Replace(config, "['**/resources/*.html']", "() => false", 1), ""},
		{"script dev override", config, ""},
		{"script build override", config, ""},
		{"script missing", config, ""},
		{"simple build", config, ""},
		{"reload", strings.Replace(config, "changed === file", "true", 1), "reload plugin"},
		{"missing reload", strings.Replace(config, "plugins: [reload]", "plugins: []", 1), "reload plugin"},
		{"missing plugin dependency", "import plugin from 'andurel-test-missing-vite-plugin'\n" + config, ""},
		{"invalid config", "export default { syntax error", "could not resolve"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Symlink(modules, filepath.Join(root, "node_modules")); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "vite.config.mjs"), []byte(tt.config), 0o600); err != nil {
				t.Fatal(err)
			}
			pkg := `{"scripts":{"dev":"vite","build":"vite build && vite build --ssr resources/js/ssr.ts --outDir assets/dist/ssr","build:client":"vite build"}}`
			switch tt.name {
			case "script dev override":
				pkg = strings.Replace(pkg, `"dev":"vite"`, `"dev":"vite --port 6123"`, 1)
			case "script build override":
				pkg = strings.Replace(pkg, `"build:client":"vite build"`, `"build:client":"vite build --base /wrong/"`, 1)
			case "script missing":
				pkg = `{"scripts":{"dev":"vite"}}`
			case "simple build":
				pkg = `{"scripts":{"dev":"vite","build":"vite build"}}`
			}
			if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(pkg), 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("node", "--input-type=module", "-", root)
			cmd.Dir = root
			devURL := "http://localhost:6123/assets/dist"
			if tt.name == "url path" {
				devURL = "http://localhost:6123/wrong"
			}
			cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + root, "NODE_ENV=test", "INERTIA_VITE_DEV_URL=" + devURL, "HOST_API=api.example:8081"}
			cmd.Stdin = strings.NewReader(string(script))
			output, err := cmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			line := strings.TrimSpace(string(output))
			if !strings.HasPrefix(line, "ANDUREL_FRONTEND_SETUP=") {
				t.Fatalf("unexpected output: %q", line)
			}
			var got struct {
				Violations          []struct{ Message string }
				Blocked             string
				EntryPoints         []string
				MissingDependencies bool
			}
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "ANDUREL_FRONTEND_SETUP=")), &got); err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(tt.name, "script ") || tt.name == "watch ignored predicate" {
				if got.Blocked == "" {
					t.Fatal("expected blocked result")
				}
				return
			}
			if tt.name == "missing plugin dependency" {
				if got.Blocked == "" || !got.MissingDependencies || len(got.Violations) != 0 {
					t.Fatalf("expected missing dependency block: %+v", got)
				}
				return
			}
			if got.Blocked != "" {
				t.Fatalf("unexpected blocked: %s", got.Blocked)
			}
			if tt.want == "" {
				if len(got.Violations) != 0 || len(got.EntryPoints) != 1 || got.EntryPoints[0] != filepath.Join(root, "resources/js/app.ts") {
					t.Fatalf("unexpected result: %+v", got)
				}
				return
			}
			for _, violation := range got.Violations {
				if strings.Contains(violation.Message, tt.want) {
					return
				}
			}
			t.Fatalf("missing %q violation: %+v", tt.want, got)
		})
	}
}

func TestFrontendSetupMissingViteDependencies(t *testing.T) {
	script, err := os.ReadFile("verify_frontend_setup.mjs")
	if err != nil {
		t.Fatal(err)
	}
	for _, broken := range []bool{false, true} {
		name := "missing vite"
		if broken {
			name = "missing vite dependency"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			write := func(name, content string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			write("package.json", `{"scripts":{"dev":"vite","build":"vite build"}}`)
			if broken {
				write("node_modules/vite/package.json", `{"type":"module","exports":"./index.js"}`)
				write("node_modules/vite/index.js", `import 'andurel-missing-dependency'`)
			}
			cmd := exec.Command("node", "--input-type=module", "-", root)
			cmd.Dir = root
			cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + root}
			cmd.Stdin = strings.NewReader(string(script))
			output, err := cmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			var got struct {
				Blocked             string
				MissingDependencies bool
			}
			line := strings.TrimSpace(string(output))
			if !strings.HasPrefix(line, "ANDUREL_FRONTEND_SETUP=") {
				t.Fatalf("unexpected output: %q", line)
			}
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "ANDUREL_FRONTEND_SETUP=")), &got); err != nil {
				t.Fatal(err)
			}
			if got.Blocked == "" || !got.MissingDependencies {
				t.Fatalf("expected missing dependency block: %+v", got)
			}
		})
	}
}
