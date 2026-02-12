package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mbvlabs/andurel/generator"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newQueriesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "queries",
		Aliases: []string{"q"},
		Short:   "SQL query management",
		Long:    "Generate and compile SQL queries for database tables.",
	}

	cmd.AddCommand(
		newQueriesGenerateCommand(),
		newQueriesRefreshCommand(),
		newQueriesCompileCommand(),
		newQueriesInitCommand(),
		newQueriesValidateCommand(),
	)

	return cmd
}

func newQueriesGenerateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate [table_name]",
		Short: "Generate CRUD queries for a database table",
		Long: `Generate SQL query file and SQLC types for a database table.
This is useful for tables that don't need a full model wrapper.

The command generates:
  - SQL queries file (database/queries/{table_name}.sql)
  - SQLC-generated query functions and types

The table name is used exactly as provided - no naming conventions are applied.
An error is returned if the table is not found in the migrations.

	Examples:
  andurel queries generate user_roles           # Generate queries for 'user_roles' table
  andurel queries generate users_organizations  # Generate queries for a junction table`,
		Args: cobra.ExactArgs(1),
		RunE: runQueriesGenerate,
	}

	return cmd
}

func runQueriesGenerate(cmd *cobra.Command, args []string) error {
	if err := chdirToProjectRoot(); err != nil {
		return err
	}

	tableName := args[0]

	gen, err := generator.New()
	if err != nil {
		return err
	}

	return gen.GenerateQueriesOnly(tableName)
}

func newQueriesRefreshCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "refresh [table_name]",
		Short: "Refresh CRUD queries for a database table",
		Long: `Refresh an existing SQL query file and SQLC types for a database table.
This keeps the queries-only file in sync with the current table schema.

Examples:
  andurel queries refresh user_roles          # Refresh queries for 'user_roles' table
  andurel queries refresh users_organizations # Refresh queries for a junction table`,
		Args: cobra.ExactArgs(1),
		RunE: runQueriesRefresh,
	}
}

func runQueriesRefresh(cmd *cobra.Command, args []string) error {
	if err := chdirToProjectRoot(); err != nil {
		return err
	}

	tableName := args[0]

	gen, err := generator.New()
	if err != nil {
		return err
	}

	return gen.RefreshQueriesOnly(tableName)
}

func newQueriesCompileCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "compile",
		Short: "Compile SQL queries and generate Go code",
		Long: `Compile SQL queries to check for errors and generate Go code.

This runs both 'sqlc compile' and 'sqlc generate' in sequence.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runSqlcCommand("compile"); err != nil {
				return err
			}
			return runSqlcCommand("generate")
		},
	}
}

func newQueriesInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize SQLC config files",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSqlcInit()
		},
	}
}

func newQueriesValidateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate database/sqlc.yaml against framework requirements",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSqlcValidate()
		},
	}
}

const (
	sqlcUserRelativePath = "database/sqlc.yaml"
	sqlcBaseRelativePath = "internal/storage/andurel_sqlc_config.yaml"
	defaultSQLCOverlay   = `# User SQLC configuration.
#
# This file is used directly by sqlc commands.
# It must preserve all required framework settings from:
# internal/storage/andurel_sqlc_config.yaml
# andurel queries validate checks this contract.
#
# You may add compatible sqlc options, but do not remove/alter required framework entries.
#
# Validate anytime with: andurel queries validate
version: "2"
sql:
  - schema: migrations
    queries: queries
    engine: postgresql
    gen:
      go:
        package: db
        out: ../models/internal/db
        output_db_file_name: db.go
        output_models_file_name: entities.go
        emit_sql_as_comment: true
        emit_methods_with_db_argument: true
        sql_package: pgx/v5
        overrides:
          - db_type: uuid
            go_type: github.com/google/uuid.UUID
`
)

type sqlcConfig struct {
	SQL []struct {
		Engine string `yaml:"engine"`
		Gen    struct {
			Go struct {
				Package                   string `yaml:"package"`
				Out                       string `yaml:"out"`
				OutputDBFileName          string `yaml:"output_db_file_name"`
				OutputModelsFileName      string `yaml:"output_models_file_name"`
				EmitMethodsWithDBArgument bool   `yaml:"emit_methods_with_db_argument"`
				SQLPackage                string `yaml:"sql_package"`
				Overrides                 []struct {
					DBType string `yaml:"db_type"`
					GoType string `yaml:"go_type"`
				} `yaml:"overrides"`
			} `yaml:"go"`
		} `yaml:"gen"`
	} `yaml:"sql"`
}

func sqlcUserPath(rootDir string) string {
	return filepath.Join(rootDir, sqlcUserRelativePath)
}

func sqlcBasePath(rootDir string) string {
	return filepath.Join(rootDir, sqlcBaseRelativePath)
}

func initQueriesSQLCFiles(rootDir string) ([]string, error) {
	basePath := sqlcBasePath(rootDir)
	if _, err := os.Stat(basePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("missing %s", basePath)
		}
		return nil, fmt.Errorf("failed to read %s: %w", basePath, err)
	}

	created := make([]string, 0, 1)
	createdUser, err := ensureSQLCFile(sqlcUserPath(rootDir), defaultSQLCOverlay)
	if err != nil {
		return nil, err
	}
	if createdUser {
		created = append(created, sqlcUserRelativePath)
	}

	if _, err := validateSQLCConfigAgainstBase(rootDir); err != nil {
		return nil, err
	}

	return created, nil
}

func validateQueriesSQLCFiles(rootDir string) error {
	_, err := validateSQLCConfigAgainstBase(rootDir)
	return err
}

func validateSQLCConfigAgainstBase(rootDir string) (string, error) {
	basePath := sqlcBasePath(rootDir)
	if _, err := os.Stat(basePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("missing %s", basePath)
		}
		return "", fmt.Errorf("failed to read base sqlc config: %w", err)
	}

	userPath := sqlcUserPath(rootDir)
	if _, err := os.Stat(userPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("missing %s; run 'andurel queries init' first", userPath)
		}
		return "", fmt.Errorf("failed to read user sqlc config: %w", err)
	}

	baseMap, err := readYAMLAsMap(basePath)
	if err != nil {
		return "", fmt.Errorf("failed to parse base sqlc config: %w", err)
	}
	userMap, err := readYAMLAsMap(userPath)
	if err != nil {
		return "", fmt.Errorf("failed to parse user sqlc config: %w", err)
	}
	if len(userMap) == 0 {
		return "", errors.New("database/sqlc.yaml cannot be empty")
	}

	issues := collectSQLCSubsetIssues(baseMap, userMap, basePath, userPath, "")
	if len(issues) > 0 {
		return "", formatSQLCValidationIssues(issues)
	}

	return userPath, nil
}

func collectSQLCSubsetIssues(base, user any, basePath, userPath, fieldPath string) []string {
	switch baseTyped := base.(type) {
	case map[string]any:
		userTyped, ok := user.(map[string]any)
		if !ok {
			return []string{fmt.Sprintf("%s must be a map", renderSQLCFieldPath(fieldPath))}
		}
		issues := make([]string, 0)
		for key, baseValue := range baseTyped {
			userValue, ok := userTyped[key]
			childPath := joinSQLCFieldPath(fieldPath, key)
			if !ok {
				issues = append(issues, fmt.Sprintf("missing required key %q in database/sqlc.yaml", childPath))
				continue
			}
			issues = append(issues, collectSQLCSubsetIssues(baseValue, userValue, basePath, userPath, childPath)...)
		}
		return issues
	case []any:
		userTyped, ok := user.([]any)
		if !ok {
			return []string{fmt.Sprintf("%s must be a list", renderSQLCFieldPath(fieldPath))}
		}
		issues := make([]string, 0)
		for _, baseValue := range baseTyped {
			bestIssues := []string{"no candidate entries found"}
			for _, userValue := range userTyped {
				candidateIssues := collectSQLCSubsetIssues(baseValue, userValue, basePath, userPath, fieldPath)
				if len(candidateIssues) == 0 {
					bestIssues = nil
					break
				}
				if len(candidateIssues) < len(bestIssues) {
					bestIssues = candidateIssues
				}
			}
			if bestIssues == nil {
				continue
			}
			issue := fmt.Sprintf(
				"missing required entry under %s from %s: %s",
				renderSQLCFieldPath(fieldPath),
				sqlcBaseRelativePath,
				summarizeSQLCYAMLValue(baseValue),
			)
			if len(bestIssues) > 0 {
				issue = issue + fmt.Sprintf(" (closest mismatch: %s)", bestIssues[0])
			}
			issues = append(issues, issue)
		}
		return issues
	default:
		if valuesEqualForSQLCField(base, user, basePath, userPath, fieldPath) {
			return nil
		}
		return []string{fmt.Sprintf(
			"required value mismatch at %s: expected %v from %s, got %v",
			renderSQLCFieldPath(fieldPath),
			base,
			sqlcBaseRelativePath,
			user,
		)}
	}
}

func formatSQLCValidationIssues(issues []string) error {
	const maxIssues = 12
	if len(issues) > maxIssues {
		remaining := len(issues) - maxIssues
		issues = append(issues[:maxIssues], fmt.Sprintf("... and %d more issue(s)", remaining))
	}
	return fmt.Errorf(
		"database/sqlc.yaml does not satisfy required settings from %s:\n- %s",
		sqlcBaseRelativePath,
		strings.Join(issues, "\n- "),
	)
}

func summarizeSQLCYAMLValue(value any) string {
	raw, err := yaml.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	summary := strings.TrimSpace(string(raw))
	summary = strings.ReplaceAll(summary, "\n", "; ")
	return summary
}

func valuesEqualForSQLCField(base, user any, basePath, userPath, fieldPath string) bool {
	baseStr, baseIsString := base.(string)
	userStr, userIsString := user.(string)
	if baseIsString && userIsString && isSQLCPathField(fieldPath) {
		baseResolved := resolveSQLCPath(baseStr, basePath)
		userResolved := resolveSQLCPath(userStr, userPath)
		return baseResolved == userResolved
	}
	return fmt.Sprint(base) == fmt.Sprint(user)
}

func isSQLCPathField(fieldPath string) bool {
	return strings.HasSuffix(fieldPath, ".schema") ||
		strings.HasSuffix(fieldPath, ".queries") ||
		strings.HasSuffix(fieldPath, ".out")
}

func resolveSQLCPath(value, configPath string) string {
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return filepath.Clean(filepath.Join(filepath.Dir(configPath), value))
}

func joinSQLCFieldPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

func renderSQLCFieldPath(fieldPath string) string {
	if fieldPath == "" {
		return "root"
	}
	return fieldPath
}

func ensureSQLCFile(path, content string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("failed to stat %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("failed to create directory for %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return false, fmt.Errorf("failed to create %s: %w", path, err)
	}
	return true, nil
}

func readYAMLAsMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return map[string]any{}, nil
	}
	result := map[string]any{}
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if result == nil {
		return map[string]any{}, nil
	}
	return result, nil
}

func validateSQLCOverlay(overlay map[string]any) error {
	if len(overlay) == 0 {
		return nil
	}
	for key := range overlay {
		if key != "sql" {
			return fmt.Errorf("database/sqlc.yaml only supports sql[].gen overlay entries; key %q is not allowed", key)
		}
	}
	rawSQL, ok := overlay["sql"]
	if !ok {
		return nil
	}
	sqlList, ok := rawSQL.([]any)
	if !ok {
		return errors.New("database/sqlc.yaml key \"sql\" must be a list")
	}
	for i, item := range sqlList {
		sqlEntry, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("database/sqlc.yaml sql[%d] must be a map", i)
		}
		for field := range sqlEntry {
			if field != "gen" {
				return fmt.Errorf("database/sqlc.yaml sql[%d].%s is not allowed; only sql[].gen is supported", i, field)
			}
		}
	}
	return nil
}

func mergeYAMLMaps(base, overlay map[string]any) map[string]any {
	result := make(map[string]any, len(base))
	for key, value := range base {
		result[key] = value
	}
	for key, overlayVal := range overlay {
		baseVal, exists := result[key]
		if !exists {
			result[key] = overlayVal
			continue
		}
		baseMap, baseIsMap := baseVal.(map[string]any)
		overlayMap, overlayIsMap := overlayVal.(map[string]any)
		if baseIsMap && overlayIsMap {
			result[key] = mergeYAMLMaps(baseMap, overlayMap)
			continue
		}
		baseSlice, baseIsSlice := baseVal.([]any)
		overlaySlice, overlayIsSlice := overlayVal.([]any)
		if baseIsSlice && overlayIsSlice {
			if key == "sql" {
				if mergedSQL, ok := mergeSQLOverlayEntries(baseSlice, overlaySlice); ok {
					result[key] = mergedSQL
					continue
				}
			}
			combined := make([]any, 0, len(baseSlice)+len(overlaySlice))
			combined = append(combined, baseSlice...)
			combined = append(combined, overlaySlice...)
			result[key] = combined
			continue
		}
		result[key] = overlayVal
	}
	return result
}

func mergeSQLOverlayEntries(baseSlice, overlaySlice []any) ([]any, bool) {
	if len(overlaySlice) == 0 {
		return baseSlice, true
	}
	if len(baseSlice) == 0 {
		return nil, false
	}

	baseFirst, ok := baseSlice[0].(map[string]any)
	if !ok {
		return nil, false
	}
	mergedFirst := mergeYAMLMaps(baseFirst, map[string]any{})
	for _, item := range overlaySlice {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, false
		}
		rawGen, hasGen := entry["gen"]
		if !hasGen {
			continue
		}
		genMap, ok := rawGen.(map[string]any)
		if !ok {
			return nil, false
		}
		currentGen, _ := mergedFirst["gen"].(map[string]any)
		if currentGen == nil {
			currentGen = map[string]any{}
		}
		mergedFirst["gen"] = mergeYAMLMaps(currentGen, genMap)
	}

	merged := make([]any, 0, len(baseSlice))
	merged = append(merged, mergedFirst)
	merged = append(merged, baseSlice[1:]...)
	return merged, true
}

func validateMergedSQLCConfig(data map[string]any) error {
	encoded, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to encode sqlc config for validation: %w", err)
	}
	var cfg sqlcConfig
	if err := yaml.Unmarshal(encoded, &cfg); err != nil {
		return fmt.Errorf("failed to parse effective sqlc config: %w", err)
	}
	if len(cfg.SQL) == 0 {
		return errors.New("effective sqlc config must include at least one sql entry")
	}
	for i, sqlCfg := range cfg.SQL {
		if sqlCfg.Engine != "postgresql" {
			return fmt.Errorf("sql[%d].engine must be postgresql", i)
		}
	}
	goCfg := cfg.SQL[0].Gen.Go
	if goCfg.Package != "db" {
		return fmt.Errorf("sql[0].gen.go.package must be db")
	}
	if goCfg.Out != "../../models/internal/db" {
		return fmt.Errorf("sql[0].gen.go.out must be ../../models/internal/db")
	}
	if goCfg.OutputDBFileName != "db.go" {
		return fmt.Errorf("sql[0].gen.go.output_db_file_name must be db.go")
	}
	if goCfg.OutputModelsFileName != "entities.go" {
		return fmt.Errorf("sql[0].gen.go.output_models_file_name must be entities.go")
	}
	if !goCfg.EmitMethodsWithDBArgument {
		return fmt.Errorf("sql[0].gen.go.emit_methods_with_db_argument must be true")
	}
	if goCfg.SQLPackage != "pgx/v5" {
		return fmt.Errorf("sql[0].gen.go.sql_package must be pgx/v5")
	}
	for i, sqlCfg := range cfg.SQL {
		for j, override := range sqlCfg.Gen.Go.Overrides {
			if override.DBType == "uuid" && override.GoType == "github.com/google/uuid.UUID" {
				continue
			}
			return fmt.Errorf("invalid override at sql[%d].gen.go.overrides[%d]: only uuid -> github.com/google/uuid.UUID is supported", i, j)
		}
	}
	return nil
}

func marshalCanonicalSQLC(data map[string]any) ([]byte, error) {
	root := &yaml.Node{Kind: yaml.MappingNode}
	appendYAMLMapping(root, "version", data["version"])
	if sqlRaw, ok := data["sql"]; ok {
		sqlNode, err := buildSQLNode(sqlRaw)
		if err != nil {
			return nil, err
		}
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "sql"}, sqlNode)
	}
	for _, key := range sortedKeysExcept(data, "version", "sql") {
		appendYAMLMapping(root, key, data[key])
	}
	doc := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}
	return yaml.Marshal(doc)
}

func buildSQLNode(value any) (*yaml.Node, error) {
	sqlList, ok := value.([]any)
	if !ok {
		return toYAMLNode(value)
	}
	seq := &yaml.Node{Kind: yaml.SequenceNode}
	for _, item := range sqlList {
		sqlMap, ok := item.(map[string]any)
		if !ok {
			node, err := toYAMLNode(item)
			if err != nil {
				return nil, err
			}
			seq.Content = append(seq.Content, node)
			continue
		}
		entry := &yaml.Node{Kind: yaml.MappingNode}
		appendYAMLIfPresent(entry, sqlMap, "engine")
		appendYAMLIfPresent(entry, sqlMap, "queries")
		appendYAMLIfPresent(entry, sqlMap, "schema")
		if genRaw, ok := sqlMap["gen"]; ok {
			genNode, err := buildGenNode(genRaw)
			if err != nil {
				return nil, err
			}
			entry.Content = append(entry.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "gen"}, genNode)
		}
		for _, key := range sortedKeysExcept(sqlMap, "engine", "queries", "schema", "gen") {
			appendYAMLMapping(entry, key, sqlMap[key])
		}
		seq.Content = append(seq.Content, entry)
	}
	return seq, nil
}

func buildGenNode(value any) (*yaml.Node, error) {
	genMap, ok := value.(map[string]any)
	if !ok {
		return toYAMLNode(value)
	}
	gen := &yaml.Node{Kind: yaml.MappingNode}
	if goRaw, ok := genMap["go"]; ok {
		goNode, err := buildGoNode(goRaw)
		if err != nil {
			return nil, err
		}
		gen.Content = append(gen.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "go"}, goNode)
	}
	for _, key := range sortedKeysExcept(genMap, "go") {
		appendYAMLMapping(gen, key, genMap[key])
	}
	return gen, nil
}

func buildGoNode(value any) (*yaml.Node, error) {
	goMap, ok := value.(map[string]any)
	if !ok {
		return toYAMLNode(value)
	}
	goNode := &yaml.Node{Kind: yaml.MappingNode}
	appendYAMLIfPresent(goNode, goMap, "package")
	appendYAMLIfPresent(goNode, goMap, "sql_package")
	appendYAMLIfPresent(goNode, goMap, "out")
	appendYAMLIfPresent(goNode, goMap, "emit_methods_with_db_argument")
	appendYAMLIfPresent(goNode, goMap, "emit_sql_as_comment")
	appendYAMLIfPresent(goNode, goMap, "output_db_file_name")
	appendYAMLIfPresent(goNode, goMap, "output_models_file_name")
	if overridesRaw, ok := goMap["overrides"]; ok {
		overridesNode, err := buildOverridesNode(overridesRaw)
		if err != nil {
			return nil, err
		}
		goNode.Content = append(goNode.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "overrides"}, overridesNode)
	}
	for _, key := range sortedKeysExcept(
		goMap,
		"package", "sql_package", "out", "emit_methods_with_db_argument",
		"emit_sql_as_comment", "output_db_file_name", "output_models_file_name", "overrides",
	) {
		appendYAMLMapping(goNode, key, goMap[key])
	}
	return goNode, nil
}

func buildOverridesNode(value any) (*yaml.Node, error) {
	list, ok := value.([]any)
	if !ok {
		return toYAMLNode(value)
	}
	seq := &yaml.Node{Kind: yaml.SequenceNode}
	for _, item := range list {
		overrideMap, ok := item.(map[string]any)
		if !ok {
			node, err := toYAMLNode(item)
			if err != nil {
				return nil, err
			}
			seq.Content = append(seq.Content, node)
			continue
		}
		entry := &yaml.Node{Kind: yaml.MappingNode}
		appendYAMLIfPresent(entry, overrideMap, "db_type")
		appendYAMLIfPresent(entry, overrideMap, "go_type")
		for _, key := range sortedKeysExcept(overrideMap, "db_type", "go_type") {
			appendYAMLMapping(entry, key, overrideMap[key])
		}
		seq.Content = append(seq.Content, entry)
	}
	return seq, nil
}

func appendYAMLMapping(node *yaml.Node, key string, value any) {
	valueNode, err := toYAMLNode(value)
	if err != nil {
		valueNode = &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprint(value)}
	}
	node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, valueNode)
}

func appendYAMLIfPresent(node *yaml.Node, data map[string]any, key string) {
	value, ok := data[key]
	if !ok {
		return
	}
	appendYAMLMapping(node, key, value)
}

func toYAMLNode(value any) (*yaml.Node, error) {
	raw, err := yaml.Marshal(value)
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 {
		return &yaml.Node{Kind: yaml.ScalarNode, Value: ""}, nil
	}
	return doc.Content[0], nil
}

func sortedKeysExcept(data map[string]any, excluded ...string) []string {
	excludeSet := make(map[string]struct{}, len(excluded))
	for _, key := range excluded {
		excludeSet[key] = struct{}{}
	}
	keys := make([]string, 0, len(data))
	for key := range data {
		if _, skip := excludeSet[key]; skip {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func runSqlcInit() error {
	rootDir, err := findGoModRoot()
	if err != nil {
		return err
	}

	created, err := initQueriesSQLCFiles(rootDir)
	if err != nil {
		return err
	}

	if len(created) == 0 {
		fmt.Fprintln(os.Stdout, "SQLC config files already exist.")
	} else {
		fmt.Fprintln(os.Stdout, "Created SQLC config files:")
		for _, path := range created {
			fmt.Fprintf(os.Stdout, "  - %s\n", path)
		}
	}

	fmt.Fprintln(os.Stdout, "SQLC base config: internal/storage/andurel_sqlc_config.yaml")
	return nil
}

func runSqlcValidate() error {
	rootDir, err := findGoModRoot()
	if err != nil {
		return err
	}

	configPath, err := validateSQLCConfigAgainstBase(rootDir)
	if err != nil {
		return err
	}

	relativePath, err := filepath.Rel(rootDir, configPath)
	if err != nil {
		relativePath = configPath
	}

	fmt.Fprintf(os.Stdout, "SQLC configuration is valid.\nRuntime config: %s\n", relativePath)
	return nil
}

func runSqlcCommand(action string) error {
	rootDir, err := findGoModRoot()
	if err != nil {
		return err
	}

	configPath, err := validateSQLCConfigAgainstBase(rootDir)
	if err != nil {
		return err
	}

	sqlcBin := filepath.Join(rootDir, "bin", "sqlc")
	if _, err := os.Stat(sqlcBin); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf(
				"sqlc binary not found at %s\nRun 'andurel tool sync' to download it",
				sqlcBin,
			)
		}
		return err
	}

	cmd := exec.Command(sqlcBin, "-f", configPath, action)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Dir = rootDir

	return cmd.Run()
}
