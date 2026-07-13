// Package extensions provides the framework for registering and applying
// extensions to the scaffold generation process.
package extensions

import (
	"embed"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/mbvlabs/andurel/layout/blueprint"
)

// Files contains the templates provided by scaffold extensions.
//
//go:embed templates/*/*.tmpl
var Files embed.FS

// TemplateData exposes scaffold data to extension templates and blueprints.
type TemplateData interface {
	DatabaseDialect() string
	GetModuleName() string
	GetInertia() string
	Builder() *blueprint.Builder
	SetBlueprint(bp *blueprint.Blueprint)
}

// ProcessTemplateFunc renders an extension template into a target file.
type ProcessTemplateFunc func(templateFile, targetPath string, data TemplateData) error

// Context carries state and callbacks for applying an extension.
type Context struct {
	TargetDir         string
	Data              TemplateData
	ProcessTemplate   ProcessTemplateFunc
	AddPostStep       func(func(targetDir string) error)
	NextMigrationTime *time.Time
	Inertia           string // inertia adapter, e.g. "vue", "react"
}

// Builder returns the blueprint builder for structured contributions.
func (ctx *Context) Builder() *blueprint.Builder {
	if ctx == nil || ctx.Data == nil {
		return nil
	}

	return ctx.Data.Builder()
}

// Extension adds files, blueprint entries, or post-processing steps to a scaffold.
type Extension interface {
	Name() string
	Apply(ctx *Context) error
	Dependencies() []string
}

var (
	registryMu sync.RWMutex
	registry   = map[string]Extension{}
)

// Register adds an extension to the global registry.
func Register(ext Extension) error {
	registryMu.Lock()
	defer registryMu.Unlock()

	if ext == nil {
		return fmt.Errorf("extensions: extension cannot be nil")
	}

	name := ext.Name()
	if name == "" {
		return fmt.Errorf("extensions: extension must provide a non-empty name")
	}

	if _, exists := registry[name]; exists {
		return nil
	}

	registry[name] = ext
	return nil
}

// Get returns a registered extension by name.
func Get(name string) (Extension, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	ext, ok := registry[name]
	return ext, ok
}

// Names returns all registered extension names in sorted order.
func Names() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()

	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
