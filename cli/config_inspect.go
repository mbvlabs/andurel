package cli

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/spf13/cobra"
)

type configKeyInfo struct {
	Key               string `json:"key"`
	Method            string `json:"method"`
	Default           string `json:"default"`
	HasDefault        bool   `json:"has_default"`
	DefaultExpression string `json:"default_expression,omitempty"`
	Required          bool   `json:"required,omitempty"`
	SourceFile        string `json:"source_file"`
	Line              int    `json:"line"`
}

type configInspection struct {
	Keys []configKeyInfo `json:"keys"`
}

type parsedConfigFile struct {
	path string
	fset *token.FileSet
	file *ast.File
}

func newConfigInspectionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "List config env keys, defaults, and source lines",
		Long: `List the environment keys read by config/*.go without running the app.

Each entry reports the key, the env reader method, the default when it can be
resolved to a literal (including package constants and duration expressions),
the default expression when it cannot, and the source file and line. Required
keys have no default.`,
		Example: `  andurel inspect config
  andurel inspect config --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}
			keys, err := collectConfigKeys(rootDir)
			if err != nil {
				return err
			}
			opts, err := output.ParseOptions(cmd)
			if err != nil {
				return err
			}
			if opts.Mode == output.ModeHuman {
				if opts.Quiet {
					return nil
				}
				return renderConfigKeysHuman(cmd.OutOrStdout(), keys)
			}
			return output.OK(
				cmd,
				configInspection{Keys: keys},
				fmt.Sprintf("Listed %d config keys", len(keys)),
			)
		},
	}
	setAgentMetadata(
		cmd,
		"introspection",
		"Read-only list of env keys read by config/*.go with defaults, source files, and lines. Use --json.",
	)
	return cmd
}

func collectConfigKeys(rootDir string) ([]configKeyInfo, error) {
	configDir := filepath.Join(rootDir, "config")
	entries, err := os.ReadDir(configDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []configKeyInfo{}, nil
		}
		return nil, err
	}

	files := make([]parsedConfigFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(configDir, entry.Name())
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("parse config file %s: %w", path, err)
		}
		files = append(files, parsedConfigFile{path: path, fset: fset, file: file})
	}
	slices.SortStableFunc(files, func(a, b parsedConfigFile) int {
		return strings.Compare(a.path, b.path)
	})

	consts := collectConfigConstants(files)
	keys := make([]configKeyInfo, 0)
	for _, file := range files {
		sourceFile := routeManifestSourceFile(rootDir, file.path)
		for _, decl := range file.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				info, ok := configKeyFromCall(call, consts, file.fset, sourceFile)
				if ok {
					keys = append(keys, info)
				}
				return true
			})
		}
	}

	slices.SortStableFunc(keys, func(a, b configKeyInfo) int {
		if a.SourceFile != b.SourceFile {
			return strings.Compare(a.SourceFile, b.SourceFile)
		}
		return a.Line - b.Line
	})
	return keys, nil
}

func collectConfigConstants(files []parsedConfigFile) map[string]ast.Expr {
	consts := map[string]ast.Expr{}
	for _, file := range files {
		for _, decl := range file.file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range valueSpec.Names {
					if i >= len(valueSpec.Values) {
						continue
					}
					consts[name.Name] = valueSpec.Values[i]
				}
			}
		}
	}
	return consts
}

func configKeyFromCall(
	call *ast.CallExpr,
	consts map[string]ast.Expr,
	fset *token.FileSet,
	sourceFile string,
) (configKeyInfo, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return configKeyInfo{}, false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok || ident.Name != "e" || !isConfigEnvReader(sel.Sel.Name) || len(call.Args) == 0 {
		return configKeyInfo{}, false
	}
	key, ok := configStringLiteral(call.Args[0])
	if !ok {
		return configKeyInfo{}, false
	}

	info := configKeyInfo{
		Key:        key,
		Method:     sel.Sel.Name,
		SourceFile: sourceFile,
		Line:       fset.Position(call.Pos()).Line,
	}
	if sel.Sel.Name == "RequiredString" {
		info.Required = true
		return info, true
	}
	if len(call.Args) < 2 {
		return info, true
	}
	if value, ok := evalConfigDefault(call.Args[1], consts); ok {
		info.Default = value
		info.HasDefault = true
		return info, true
	}
	info.DefaultExpression = formatConfigExpr(fset, call.Args[1])
	return info, true
}

func isConfigEnvReader(name string) bool {
	switch name {
	case "String", "RequiredString", "Strings",
		"Int", "Int32", "Int64", "Bool", "Float64", "Duration":
		return true
	default:
		return false
	}
}

func configStringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return value, true
}

func evalConfigDefault(expr ast.Expr, consts map[string]ast.Expr) (string, bool) {
	resolved := resolveConfigConstExpr(expr, consts, map[string]struct{}{})
	if containsConfigTimeUnit(resolved) {
		if duration, ok := evalConfigDuration(resolved, consts, map[string]struct{}{}); ok {
			return duration.String(), true
		}
	}
	return evalConfigValue(expr, consts, map[string]struct{}{})
}

func resolveConfigConstExpr(
	expr ast.Expr,
	consts map[string]ast.Expr,
	seen map[string]struct{},
) ast.Expr {
	for {
		switch typed := expr.(type) {
		case *ast.ParenExpr:
			expr = typed.X
		case *ast.Ident:
			value, ok := consts[typed.Name]
			if !ok {
				return expr
			}
			if _, loop := seen[typed.Name]; loop {
				return expr
			}
			seen[typed.Name] = struct{}{}
			expr = value
		default:
			return expr
		}
	}
}

func evalConfigValue(
	expr ast.Expr,
	consts map[string]ast.Expr,
	seen map[string]struct{},
) (string, bool) {
	switch typed := expr.(type) {
	case *ast.BasicLit:
		switch typed.Kind {
		case token.STRING:
			value, err := strconv.Unquote(typed.Value)
			return value, err == nil
		case token.INT, token.FLOAT:
			return typed.Value, true
		}
	case *ast.Ident:
		switch typed.Name {
		case "true", "false":
			return typed.Name, true
		case "nil":
			return "", true
		}
		if _, loop := seen[typed.Name]; loop {
			return "", false
		}
		value, ok := consts[typed.Name]
		if !ok {
			return "", false
		}
		seen[typed.Name] = struct{}{}
		return evalConfigValue(value, consts, seen)
	case *ast.UnaryExpr:
		inner, ok := evalConfigValue(typed.X, consts, seen)
		if !ok {
			return "", false
		}
		if typed.Op == token.SUB {
			return "-" + inner, true
		}
		return inner, true
	case *ast.BinaryExpr:
		if typed.Op != token.ADD {
			return "", false
		}
		left, leftOK := evalConfigValue(typed.X, consts, seen)
		right, rightOK := evalConfigValue(typed.Y, consts, seen)
		if !leftOK || !rightOK {
			return "", false
		}
		return left + right, true
	case *ast.ParenExpr:
		return evalConfigValue(typed.X, consts, seen)
	case *ast.CallExpr:
		// int64(...) and friends are conversions, not calls with side effects.
		if len(typed.Args) == 1 {
			if ident, ok := typed.Fun.(*ast.Ident); ok {
				switch ident.Name {
				case "int", "int32", "int64", "float64":
					return evalConfigValue(typed.Args[0], consts, seen)
				}
			}
		}
	}
	return "", false
}

func containsConfigTimeUnit(expr ast.Expr) bool {
	switch typed := expr.(type) {
	case *ast.SelectorExpr:
		ident, ok := typed.X.(*ast.Ident)
		return ok && ident.Name == "time"
	case *ast.BinaryExpr:
		return containsConfigTimeUnit(typed.X) || containsConfigTimeUnit(typed.Y)
	case *ast.ParenExpr:
		return containsConfigTimeUnit(typed.X)
	default:
		return false
	}
}

func evalConfigDuration(
	expr ast.Expr,
	consts map[string]ast.Expr,
	seen map[string]struct{},
) (time.Duration, bool) {
	switch typed := expr.(type) {
	case *ast.BasicLit:
		if typed.Kind != token.INT {
			return 0, false
		}
		value, err := strconv.ParseInt(typed.Value, 10, 64)
		if err != nil {
			return 0, false
		}
		return time.Duration(value), true
	case *ast.Ident:
		value, ok := consts[typed.Name]
		if !ok {
			return 0, false
		}
		if _, loop := seen[typed.Name]; loop {
			return 0, false
		}
		seen[typed.Name] = struct{}{}
		return evalConfigDuration(value, consts, seen)
	case *ast.SelectorExpr:
		ident, ok := typed.X.(*ast.Ident)
		if !ok || ident.Name != "time" {
			return 0, false
		}
		switch typed.Sel.Name {
		case "Nanosecond":
			return time.Nanosecond, true
		case "Microsecond":
			return time.Microsecond, true
		case "Millisecond":
			return time.Millisecond, true
		case "Second":
			return time.Second, true
		case "Minute":
			return time.Minute, true
		case "Hour":
			return time.Hour, true
		default:
			return 0, false
		}
	case *ast.ParenExpr:
		return evalConfigDuration(typed.X, consts, seen)
	case *ast.BinaryExpr:
		switch typed.Op {
		case token.MUL:
			if left, ok := evalConfigNumber(typed.X, consts, seen); ok {
				if right, ok := evalConfigDuration(typed.Y, consts, seen); ok {
					return time.Duration(left) * right, true
				}
			}
			if right, ok := evalConfigNumber(typed.Y, consts, seen); ok {
				if left, ok := evalConfigDuration(typed.X, consts, seen); ok {
					return left * time.Duration(right), true
				}
			}
		case token.ADD:
			left, leftOK := evalConfigDuration(typed.X, consts, seen)
			right, rightOK := evalConfigDuration(typed.Y, consts, seen)
			if leftOK && rightOK {
				return left + right, true
			}
		}
	}
	return 0, false
}

func evalConfigNumber(
	expr ast.Expr,
	consts map[string]ast.Expr,
	seen map[string]struct{},
) (int64, bool) {
	switch typed := expr.(type) {
	case *ast.BasicLit:
		if typed.Kind != token.INT {
			return 0, false
		}
		value, err := strconv.ParseInt(typed.Value, 10, 64)
		return value, err == nil
	case *ast.Ident:
		value, ok := consts[typed.Name]
		if !ok {
			return 0, false
		}
		if _, loop := seen[typed.Name]; loop {
			return 0, false
		}
		seen[typed.Name] = struct{}{}
		return evalConfigNumber(value, consts, seen)
	case *ast.ParenExpr:
		return evalConfigNumber(typed.X, consts, seen)
	case *ast.UnaryExpr:
		value, ok := evalConfigNumber(typed.X, consts, seen)
		if !ok {
			return 0, false
		}
		if typed.Op == token.SUB {
			return -value, true
		}
		return value, true
	case *ast.BinaryExpr:
		left, leftOK := evalConfigNumber(typed.X, consts, seen)
		right, rightOK := evalConfigNumber(typed.Y, consts, seen)
		if !leftOK || !rightOK {
			return 0, false
		}
		switch typed.Op {
		case token.ADD:
			return left + right, true
		case token.SUB:
			return left - right, true
		case token.MUL:
			return left * right, true
		}
	}
	return 0, false
}

func formatConfigExpr(fset *token.FileSet, expr ast.Expr) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, expr); err != nil {
		return ""
	}
	return buf.String()
}

func renderConfigKeysHuman(w io.Writer, keys []configKeyInfo) error {
	if len(keys) == 0 {
		_, err := fmt.Fprintln(w, "No config keys found.")
		return err
	}
	table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "KEY\tMETHOD\tDEFAULT\tSOURCE"); err != nil {
		return err
	}
	for _, key := range keys {
		defaultValue := "-"
		switch {
		case key.Required:
			defaultValue = "required"
		case key.HasDefault && key.Default == "":
			defaultValue = `""`
		case key.HasDefault:
			defaultValue = key.Default
		case key.DefaultExpression != "":
			defaultValue = key.DefaultExpression
		}
		if _, err := fmt.Fprintf(
			table,
			"%s\t%s\t%s\t%s:%d\n",
			key.Key,
			key.Method,
			defaultValue,
			key.SourceFile,
			key.Line,
		); err != nil {
			return err
		}
	}
	return table.Flush()
}
