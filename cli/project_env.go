package cli

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/joho/godotenv"
)

// configValueSource records where an effective configuration value came from
// so inspect urls/process can report configuration provenance without leaking
// unrelated environment values.
type configValueSource struct {
	Key        string `json:"key"`
	Value      string `json:"value,omitempty"`
	Source     string `json:"source"`
	SourceFile string `json:"source_file,omitempty"`
	SourceLine int    `json:"source_line,omitempty"`
	Found      bool   `json:"found"`
}

func (source configValueSource) describe() string {
	if !source.Found {
		return source.Key + " is not set in the environment, .env, or config defaults"
	}
	switch source.Source {
	case "environment":
		return source.Key + " from exported environment"
	case ".env":
		return source.Key + " from .env"
	case "config-default":
		if source.SourceFile != "" {
			return source.Key + " default from " + source.SourceFile
		}
		return source.Key + " from a config default"
	default:
		return source.Key + " from " + source.Source
	}
}

type projectEnvResolver struct {
	rootDir  string
	exported map[string]string
	dotenv   map[string]string
	defaults map[string]configKeyInfo
}

func newProjectEnvResolver(rootDir string) projectEnvResolver {
	resolver := projectEnvResolver{
		rootDir:  rootDir,
		exported: map[string]string{},
		dotenv:   map[string]string{},
		defaults: map[string]configKeyInfo{},
	}
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			resolver.exported[key] = value
		}
	}
	if values, err := godotenv.Read(filepath.Join(rootDir, ".env")); err == nil {
		resolver.dotenv = values
	}
	if keys, err := collectConfigKeys(rootDir); err == nil {
		for _, key := range keys {
			resolver.defaults[key.Key] = key
		}
	}
	return resolver
}

func (resolver projectEnvResolver) resolve(key string) configValueSource {
	if value, ok := resolver.exported[key]; ok {
		return configValueSource{Key: key, Value: value, Source: "environment", Found: true}
	}
	if value, ok := resolver.dotenv[key]; ok {
		return configValueSource{Key: key, Value: value, Source: ".env", Found: true}
	}
	if keyInfo, ok := resolver.defaults[key]; ok && keyInfo.HasDefault && !keyInfo.Required {
		return configValueSource{
			Key:        key,
			Value:      keyInfo.Default,
			Source:     "config-default",
			SourceFile: keyInfo.SourceFile,
			SourceLine: keyInfo.Line,
			Found:      true,
		}
	}
	return configValueSource{Key: key}
}

// protocol resolves PROTOCOL, falling back to the http default in
// config.App.BaseURL.
func (resolver projectEnvResolver) protocol() configValueSource {
	source := resolver.resolve("PROTOCOL")
	if source.Found && strings.TrimSpace(source.Value) != "" {
		return source
	}
	return configValueSource{
		Key:    "PROTOCOL",
		Value:  "http",
		Source: "config-default",
		Found:  true,
	}
}

func hostConfigKey(host string) string {
	if host == "" || host == string(mustHostPrimary()) {
		return "HOST_PRIMARY"
	}
	var builder strings.Builder
	builder.WriteString("HOST_")
	for _, r := range host {
		switch {
		case unicode.IsLetter(r):
			builder.WriteRune(unicode.ToUpper(r))
		case unicode.IsDigit(r):
			builder.WriteRune(r)
		default:
			builder.WriteRune('_')
		}
	}
	return builder.String()
}
