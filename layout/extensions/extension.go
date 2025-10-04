package extensions

import (
	"embed"
	"fmt"
	"sort"
	"sync"
)

//go:embed */templates/*.tmpl
var Files embed.FS

type TemplateData interface {
	AddSlotSnippet(slot, snippet string) error
	AddSlotData(slot string, value any) error
	Slot(slot string) []string
	SlotJoined(slot, sep string) string
	SlotData(slot string) []any
	SlotNames() []string
	HasSlot(slot string) bool
	HasSlotData(slot string) bool
	DatabaseDialect() string
}

type ProcessTemplateFunc func(templateFile, targetPath string, data TemplateData) error

type Context struct {
	TargetDir       string
	Data            TemplateData
	ProcessTemplate ProcessTemplateFunc
	AddPostStep     func(func() error)
}

// AddSlotSnippet appends a snippet to the targeted slot using the context's
// template data.
func (ctx *Context) AddSlotSnippet(slot, snippet string) error {
	if ctx == nil {
		return fmt.Errorf("extensions: context is nil")
	}

	if ctx.Data == nil {
		return fmt.Errorf("extensions: template data is nil")
	}

	return ctx.Data.AddSlotSnippet(slot, snippet)
}

// AddSlotData appends a structured value to the targeted slot using the
// context's template data.
func (ctx *Context) AddSlotData(slot string, value any) error {
	if ctx == nil {
		return fmt.Errorf("extensions: context is nil")
	}

	if ctx.Data == nil {
		return fmt.Errorf("extensions: template data is nil")
	}

	return ctx.Data.AddSlotData(slot, value)
}

type Extension interface {
	Name() string
	Apply(ctx *Context) error
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
