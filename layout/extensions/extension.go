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

//go:embed templates/*/*.tmpl
var Files embed.FS

type TemplateData interface {
	DatabaseDialect() string
	GetModuleName() string
	Builder() *blueprint.Builder
	SetBlueprint(bp *blueprint.Blueprint)
}

type ProcessTemplateFunc func(templateFile, targetPath string, data TemplateData) error

type Context struct {
	TargetDir         string
	Data              TemplateData
	ProcessTemplate   ProcessTemplateFunc
	AddPostStep       func(func(targetDir string) error)
	NextMigrationTime *time.Time
}

// Builder returns the blueprint builder for structured contributions.
func (ctx *Context) Builder() *blueprint.Builder {
	if ctx == nil || ctx.Data == nil {
		return nil
	}

	return ctx.Data.Builder()
}

type Extension interface {
	Name() string
	Apply(ctx *Context) error
	Dependencies() []string
}

var (
	registryMu sync.RWMutex
	registry   = map[string]Extension{}
)

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

func Get(name string) (Extension, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	ext, ok := registry[name]
	return ext, ok
}

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
