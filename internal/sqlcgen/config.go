package sqlcgen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sqlc-dev/plugin-sdk-go/plugin"
)

// Config is the resolved configuration for a single plugin invocation.
//
// Module is the importing project's Go module path (read from go.mod walking
// up from the working directory). It's needed so emitted files can reference
// the project's internal/storage and models/internal/db packages.
//
// Package is the package name to emit (defaults to "models").
//
// DBPackageImport is the import path of the sqlc-generated row package
// (defaults to <Module>/models/internal/db).
//
// StoragePackageImport is the import path of the andurel storage runtime
// (defaults to <Module>/internal/storage).
type Config struct {
	Module               string
	Package              string
	DBPackageImport      string
	StoragePackageImport string
}

// pluginOptions mirrors the JSON shape under codegen[].options in sqlc.yaml.
// All fields are optional; sensible defaults are derived from the project's
// go.mod when unset.
type pluginOptions struct {
	Module               string `json:"module,omitempty"`
	Package              string `json:"package,omitempty"`
	DBPackageImport      string `json:"db_package_import,omitempty"`
	StoragePackageImport string `json:"storage_package_import,omitempty"`
}

func loadConfig(req *plugin.GenerateRequest) (*Config, error) {
	var opts pluginOptions
	if raw := req.GetSettings().GetCodegen().GetOptions(); len(raw) > 0 {
		if err := json.Unmarshal(raw, &opts); err != nil {
			return nil, fmt.Errorf("unmarshal plugin options: %w", err)
		}
	}

	module := opts.Module
	if module == "" {
		discovered, err := discoverModule()
		if err != nil {
			return nil, fmt.Errorf("discover module from go.mod (set options.module to override): %w", err)
		}
		module = discovered
	}

	cfg := &Config{
		Module:               module,
		Package:              firstNonEmpty(opts.Package, "models"),
		DBPackageImport:      firstNonEmpty(opts.DBPackageImport, module+"/models/internal/db"),
		StoragePackageImport: firstNonEmpty(opts.StoragePackageImport, module+"/internal/storage"),
	}
	return cfg, nil
}

// discoverModule walks up from cwd looking for go.mod and reads the module
// directive. sqlc invokes plugins with cwd set to the directory containing
// sqlc.yaml, which is typically nested under the project root.
func discoverModule() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		path := filepath.Join(dir, "go.mod")
		if data, err := os.ReadFile(path); err == nil {
			return parseModuleLine(string(data))
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}

func parseModuleLine(content string) (string, error) {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module")), nil
		}
	}
	return "", fmt.Errorf("module directive not found in go.mod")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
