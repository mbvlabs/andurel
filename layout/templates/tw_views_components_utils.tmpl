package components

import (
	"strings"

	"github.com/a-h/templ"
	"github.com/rs/xid"
)

func cn(classes ...string) string {
	var filtered []string
	for _, c := range classes {
		c = strings.TrimSpace(c)
		if c != "" {
			filtered = append(filtered, c)
		}
	}
	return strings.Join(filtered, " ")
}

type Opt func(*optConfig)

type optConfig struct {
	class         string
	classOverride bool
	attrs         templ.Attributes
	id            string
	fragmentID    string
}

func WithClass(c string) Opt {
	return func(cfg *optConfig) { cfg.class = cn(cfg.class, c) }
}

func SetClass(c string) Opt {
	return func(cfg *optConfig) { cfg.class = c; cfg.classOverride = true }
}

func WithAttr(key string, val any) Opt {
	return func(cfg *optConfig) {
		if cfg.attrs == nil {
			cfg.attrs = templ.Attributes{}
		}
		cfg.attrs[key] = val
	}
}

func WithID(id string) Opt {
	return func(cfg *optConfig) { cfg.id = id }
}

func WithFragment(id string) Opt {
	return func(cfg *optConfig) { cfg.fragmentID = id }
}

func fragmentFrom(opts []Opt) string {
	cfg := optConfig{}
	for _, o := range opts {
		o(&cfg)
	}
	return cfg.fragmentID
}

func classFrom(opts []Opt) string {
	cfg := optConfig{}
	for _, o := range opts {
		o(&cfg)
	}
	return cfg.class
}

func classOverrideFrom(opts []Opt) bool {
	cfg := optConfig{}
	for _, o := range opts {
		o(&cfg)
	}
	return cfg.classOverride
}

func mergeClass(base string, class string, override bool) string {
	if override {
		return strings.TrimSpace(class)
	}
	return cn(base, class)
}

func attrsFrom(opts []Opt) templ.Attributes {
	cfg := optConfig{}
	for _, o := range opts {
		o(&cfg)
	}
	return cfg.attrs
}

func idFrom(opts []Opt) string {
	cfg := optConfig{}
	for _, o := range opts {
		o(&cfg)
	}
	return cfg.id
}

func resolveID(opts []Opt) string {
	if id := idFrom(opts); id != "" {
		return id
	}
	return xid.New().String()
}

func triggerClickExpr(id string, attrs templ.Attributes) string {
	if attrs != nil {
		if v, ok := attrs["data-on:click"]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return "$_" + id + " = !$_" + id
}

func attrsWithout(attrs templ.Attributes, keys ...string) templ.Attributes {
	if attrs == nil {
		return nil
	}
	skip := make(map[string]bool, len(keys))
	for _, k := range keys {
		skip[k] = true
	}
	out := templ.Attributes{}
	for k, v := range attrs {
		if !skip[k] {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
