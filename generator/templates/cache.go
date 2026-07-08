package templates

import (
	"sync"
	"text/template"
)

// TemplateCache represents template cache.
type TemplateCache struct {
	templates map[string]*template.Template
	mutex     sync.RWMutex
}

// NewTemplateCache creates a new template cache.
func NewTemplateCache() *TemplateCache {
	return &TemplateCache{
		templates: make(map[string]*template.Template),
	}
}

// GetTemplate returns template.
func (tc *TemplateCache) GetTemplate(
	templateName string,
	funcMap template.FuncMap,
) (*template.Template, error) {
	cacheKey := templateName

	tc.mutex.RLock()
	if cachedTmpl, exists := tc.templates[cacheKey]; exists {
		tc.mutex.RUnlock()
		return cachedTmpl, nil
	}
	tc.mutex.RUnlock()

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	if cachedTmpl, exists := tc.templates[cacheKey]; exists {
		return cachedTmpl, nil
	}

	templateContent, err := Files.ReadFile(templateName)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New(templateName).Funcs(funcMap).Parse(string(templateContent))
	if err != nil {
		return nil, err
	}

	tc.templates[cacheKey] = tmpl
	return tmpl, nil
}

var globalCache = NewTemplateCache()

// GetCachedTemplate returns cached template.
func GetCachedTemplate(templateName string, funcMap template.FuncMap) (*template.Template, error) {
	return globalCache.GetTemplate(templateName, funcMap)
}

// ClearCache clears cache.
func ClearCache() {
	globalCache.mutex.Lock()
	defer globalCache.mutex.Unlock()
	globalCache.templates = make(map[string]*template.Template)
}
