package cli

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mbvlabs/andurel/cli/output"
	"github.com/spf13/cobra"
)

type agentConfig struct {
	PreferredGeneratorMode       string            `json:"preferred_generator_mode,omitempty"`
	JavaScriptRuntime            string            `json:"javascript_runtime,omitempty"`
	DefaultNamespace             string            `json:"default_namespace,omitempty"`
	OutputFormat                 string            `json:"output_format,omitempty"`
	CommonDatabaseCommandOptions map[string]string `json:"common_database_command_options,omitempty"`
	Values                       map[string]string `json:"values,omitempty"`
}

type configShowReport struct {
	User    agentConfig `json:"user"`
	Project agentConfig `json:"project"`
	Cache   agentConfig `json:"cache"`
	Merged  agentConfig `json:"merged"`
	Paths   configPaths `json:"paths"`
}

type configPaths struct {
	User    string `json:"user"`
	Project string `json:"project"`
	Cache   string `json:"cache"`
}

func newConfigCommand() *cobra.Command {
	var scope string
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage Andurel agent configuration",
		Long:  "Manage user, project, and cache configuration for agent-friendly Andurel defaults.",
	}
	setAgentMetadata(cmd, "config", "Reads and writes non-secret Andurel configuration.")
	cmd.PersistentFlags().StringVar(&scope, "scope", "project", "Config scope: project, user, or cache")

	cmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Initialize config files",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := configPath(scope)
			if err != nil {
				return err
			}
			cfg := defaultAgentConfig()
			if err := writeAgentConfig(path, cfg); err != nil {
				return err
			}
			return output.OK(cmd, map[string]string{"path": path, "scope": scope}, "Initialized Andurel config")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show config",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			report, err := loadConfigShowReport()
			if err != nil {
				return err
			}
			return output.OK(cmd, report, "Loaded Andurel config")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "set KEY VALUE",
		Short: "Set a config value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := configPath(scope)
			if err != nil {
				return err
			}
			cfg, err := readOptionalAgentConfig(path)
			if err != nil {
				return err
			}
			setConfigValue(&cfg, args[0], args[1])
			if err := writeAgentConfig(path, cfg); err != nil {
				return err
			}
			return output.OK(cmd, map[string]string{"scope": scope, "key": args[0], "value": args[1]}, "Updated Andurel config")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "unset KEY",
		Short: "Unset a config value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := configPath(scope)
			if err != nil {
				return err
			}
			cfg, err := readOptionalAgentConfig(path)
			if err != nil {
				return err
			}
			unsetConfigValue(&cfg, args[0])
			if err := writeAgentConfig(path, cfg); err != nil {
				return err
			}
			return output.OK(cmd, map[string]string{"scope": scope, "key": args[0]}, "Updated Andurel config")
		},
	})

	return cmd
}

func defaultAgentConfig() agentConfig {
	return agentConfig{
		PreferredGeneratorMode: "safe",
		JavaScriptRuntime:      "npm",
		OutputFormat:           "human",
		CommonDatabaseCommandOptions: map[string]string{
			"migrate": "up",
		},
		Values: map[string]string{},
	}
}

func loadConfigShowReport() (configShowReport, error) {
	userPath, _ := userConfigPath()
	projectPath, _ := projectConfigPath()
	cachePath, _ := cacheConfigPath()
	user, err := readOptionalAgentConfig(userPath)
	if err != nil {
		return configShowReport{}, err
	}
	project, err := readOptionalAgentConfig(projectPath)
	if err != nil {
		return configShowReport{}, err
	}
	cacheCfg, err := readOptionalAgentConfig(cachePath)
	if err != nil {
		return configShowReport{}, err
	}
	merged := mergeAgentConfigs(user, cacheCfg, project)
	return configShowReport{
		User:    user,
		Project: project,
		Cache:   cacheCfg,
		Merged:  merged,
		Paths: configPaths{
			User:    userPath,
			Project: projectPath,
			Cache:   cachePath,
		},
	}, nil
}

func configPath(scope string) (string, error) {
	switch strings.ToLower(scope) {
	case "project":
		return projectConfigPath()
	case "user":
		return userConfigPath()
	case "cache":
		return cacheConfigPath()
	default:
		return "", output.NewError(output.CodeUsage, "invalid config scope: "+scope, output.ExitUsage, "Use project, user, or cache.")
	}
}

func projectConfigPath() (string, error) {
	rootDir, err := findGoModRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(rootDir, ".andurel", "config.json"), nil
}

func userConfigPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "andurel", "config.json"), nil
}

func userCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "andurel"), nil
}

func cacheConfigPath() (string, error) {
	base, err := userCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "config.json"), nil
}

func readAgentConfig(path string) (agentConfig, error) {
	cfg := agentConfig{Values: map[string]string{}}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("failed to parse config %s: %w", path, err)
	}
	if cfg.Values == nil {
		cfg.Values = map[string]string{}
	}
	return cfg, nil
}

func readOptionalAgentConfig(path string) (agentConfig, error) {
	cfg, err := readAgentConfig(path)
	if err == nil {
		return cfg, nil
	}
	if path == "" || os.IsNotExist(err) {
		return agentConfig{Values: map[string]string{}}, nil
	}
	return agentConfig{}, err
}

func writeAgentConfig(path string, cfg agentConfig) error {
	if cfg.Values == nil {
		cfg.Values = map[string]string{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func mergeAgentConfigs(configs ...agentConfig) agentConfig {
	merged := agentConfig{Values: map[string]string{}, CommonDatabaseCommandOptions: map[string]string{}}
	for _, cfg := range configs {
		if cfg.PreferredGeneratorMode != "" {
			merged.PreferredGeneratorMode = cfg.PreferredGeneratorMode
		}
		if cfg.JavaScriptRuntime != "" {
			merged.JavaScriptRuntime = cfg.JavaScriptRuntime
		}
		if cfg.DefaultNamespace != "" {
			merged.DefaultNamespace = cfg.DefaultNamespace
		}
		if cfg.OutputFormat != "" {
			merged.OutputFormat = cfg.OutputFormat
		}
		maps.Copy(merged.CommonDatabaseCommandOptions, cfg.CommonDatabaseCommandOptions)
		maps.Copy(merged.Values, cfg.Values)
	}
	return merged
}

func setConfigValue(cfg *agentConfig, key, value string) {
	switch key {
	case "preferred_generator_mode":
		cfg.PreferredGeneratorMode = value
	case "javascript_runtime":
		cfg.JavaScriptRuntime = value
	case "default_namespace":
		cfg.DefaultNamespace = value
	case "output_format":
		cfg.OutputFormat = value
	default:
		if cfg.Values == nil {
			cfg.Values = map[string]string{}
		}
		cfg.Values[key] = value
	}
}

func unsetConfigValue(cfg *agentConfig, key string) {
	switch key {
	case "preferred_generator_mode":
		cfg.PreferredGeneratorMode = ""
	case "javascript_runtime":
		cfg.JavaScriptRuntime = ""
	case "default_namespace":
		cfg.DefaultNamespace = ""
	case "output_format":
		cfg.OutputFormat = ""
	default:
		delete(cfg.Values, key)
	}
}

func sortedConfigKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
