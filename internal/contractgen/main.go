// Command contractgen emits deterministic release contract fixtures.
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/mbvlabs/andurel/cli"
	"github.com/mbvlabs/andurel/cli/output"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const modulePath = "github.com/mbvlabs/andurel"

type contract struct {
	SchemaVersion int                  `json:"schema_version"`
	Commands      []commandContract    `json:"commands"`
	JSONStructs   []jsonStructContract `json:"json_structs"`
	Success       wireContract         `json:"success_envelope"`
	Failure       wireContract         `json:"error_envelope"`
	Projections   []projectionContract `json:"projections"`
	Errors        []errorContract      `json:"errors"`
}

type commandContract struct {
	Path    string         `json:"path"`
	Use     string         `json:"use"`
	Aliases []string       `json:"aliases,omitempty"`
	Flags   []flagContract `json:"flags,omitempty"`
}

type flagContract struct {
	Name       string `json:"name"`
	Shorthand  string `json:"shorthand,omitempty"`
	Type       string `json:"type"`
	Default    string `json:"default"`
	Persistent bool   `json:"persistent,omitempty"`
}

type jsonStructContract struct {
	Type   string              `json:"type"`
	Fields []jsonFieldContract `json:"fields"`
}

type jsonFieldContract struct {
	GoName    string `json:"go_name"`
	JSONName  string `json:"json_name"`
	OmitEmpty bool   `json:"omitempty,omitempty"`
}

type wireContract struct {
	Fields []jsonFieldContract `json:"fields"`
}

type projectionContract struct {
	Flag  string `json:"flag"`
	Shape string `json:"shape"`
}

type errorContract struct {
	Code     string `json:"code"`
	ExitCode int    `json:"exit_code"`
}

func main() {
	if len(os.Args) != 2 {
		fatalf("usage: contractgen packages|cli")
	}

	var err error
	switch os.Args[1] {
	case "packages":
		err = emitPackages()
	case "cli":
		err = emitCLIContract()
	default:
		fatalf("unknown contract: %s", os.Args[1])
	}
	if err != nil {
		fatalf("%v", err)
	}
}

func emitPackages() error {
	root, err := repositoryRoot()
	if err != nil {
		return err
	}

	packages := map[string]struct{}{}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && shouldSkipDirectory(root, path, entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(entry.Name(), "_test.go") || !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}

		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.PackageClauseOnly)
		if err != nil {
			return fmt.Errorf("parse package clause %s: %w", path, err)
		}
		if file.Name.Name == "main" {
			return nil
		}

		dir := filepath.Dir(path)
		rel, err := filepath.Rel(root, dir)
		if err != nil {
			return err
		}
		packages[modulePath+"/"+filepath.ToSlash(rel)] = struct{}{}
		return nil
	})
	if err != nil {
		return err
	}

	names := make([]string, 0, len(packages))
	for name := range packages {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		fmt.Println(name)
	}
	return nil
}

func shouldSkipDirectory(root, path, name string) bool {
	if strings.HasPrefix(name, ".") || name == "testdata" || name == "vendor" || name == "e2e" {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return slices.Contains(strings.Split(filepath.ToSlash(rel), "/"), "internal")
}

func emitCLIContract() error {
	root := cli.NewRootCommand("contract", "contract")
	commands := collectCommands(root)
	jsonStructs, err := collectJSONStructs()
	if err != nil {
		return err
	}

	value := contract{
		SchemaVersion: 1,
		Commands:      commands,
		JSONStructs:   jsonStructs,
		Success:       wireContract{Fields: jsonFields(reflect.TypeFor[output.Envelope]())},
		Failure:       wireContract{Fields: jsonFields(reflect.TypeFor[output.ErrorEnvelope]())},
		Projections: []projectionContract{
			{Flag: "--count", Shape: "one raw base-10 integer followed by a newline"},
			{Flag: "--ids-only", Shape: "one raw identifier per line"},
			{Flag: "--jq", Shape: "the selected data payload value encoded directly as JSON"},
		},
		Errors: []errorContract{
			{Code: output.CodeError, ExitCode: output.ExitUsage},
			{Code: output.CodeUsage, ExitCode: output.ExitUsage},
			{Code: output.CodeOutputMode, ExitCode: output.ExitUsage},
			{Code: output.CodeProjectNotFound, ExitCode: output.ExitProject},
			{Code: output.CodeMissingTool, ExitCode: output.ExitDependency},
			{Code: output.CodeInvalidExtension, ExitCode: output.ExitUsage},
			{Code: output.CodeInvalidInertiaAdapter, ExitCode: output.ExitUsage},
			{Code: output.CodeUnsafeAction, ExitCode: output.ExitUnsafe},
			{Code: output.CodeGenerationFailed, ExitCode: output.ExitGeneration},
			{Code: output.CodeExternalCommandFailed, ExitCode: output.ExitExternal},
			{Code: output.CodeConfigError, ExitCode: output.ExitConfig},
			{Code: output.CodeAmbiguousInput, ExitCode: output.ExitAmbiguous},
			{Code: output.CodeUpdateRequired, ExitCode: output.ExitDependency},
		},
	}
	slices.SortFunc(value.Errors, func(a, b errorContract) int { return strings.Compare(a.Code, b.Code) })

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func collectCommands(root *cobra.Command) []commandContract {
	var commands []commandContract
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		cmd.InitDefaultHelpFlag()
		if cmd == root {
			cmd.InitDefaultVersionFlag()
		}

		flags := append(flagsFromSet(cmd.LocalNonPersistentFlags(), false), flagsFromSet(cmd.PersistentFlags(), true)...)
		slices.SortFunc(flags, func(a, b flagContract) int { return strings.Compare(a.Name, b.Name) })
		commands = append(commands, commandContract{
			Path:    cmd.CommandPath(),
			Use:     cmd.Use,
			Aliases: append([]string(nil), cmd.Aliases...),
			Flags:   flags,
		})

		children := make([]*cobra.Command, 0)
		for _, child := range cmd.Commands() {
			if child.Hidden || !child.IsAvailableCommand() {
				continue
			}
			children = append(children, child)
		}
		slices.SortFunc(children, func(a, b *cobra.Command) int { return strings.Compare(a.Name(), b.Name()) })
		for _, child := range children {
			walk(child)
		}
	}
	walk(root)
	slices.SortFunc(commands, func(a, b commandContract) int { return strings.Compare(a.Path, b.Path) })
	return commands
}

func flagsFromSet(set *pflag.FlagSet, persistent bool) []flagContract {
	if set == nil {
		return nil
	}
	var flags []flagContract
	set.VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		flags = append(flags, flagContract{
			Name:       flag.Name,
			Shorthand:  flag.Shorthand,
			Type:       flag.Value.Type(),
			Default:    flag.DefValue,
			Persistent: persistent,
		})
	})
	return flags
}

func collectJSONStructs() ([]jsonStructContract, error) {
	root, err := repositoryRoot()
	if err != nil {
		return nil, err
	}

	directories := []string{"cli", "generator", "layout"}
	var contracts []jsonStructContract
	for _, directory := range directories {
		base := filepath.Join(root, directory)
		err := filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if entry.Name() == "internal" || entry.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(entry.Name(), "_test.go") || !strings.HasSuffix(entry.Name(), ".go") {
				return nil
			}
			return collectFileJSONStructs(root, path, &contracts)
		})
		if err != nil {
			return nil, err
		}
	}

	slices.SortFunc(contracts, func(a, b jsonStructContract) int { return strings.Compare(a.Type, b.Type) })
	return contracts, nil
}

func collectFileJSONStructs(root, path string, contracts *[]jsonStructContract) error {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	relDir, err := filepath.Rel(root, filepath.Dir(path))
	if err != nil {
		return err
	}
	pkg := modulePath + "/" + filepath.ToSlash(relDir)

	ast.Inspect(file, func(node ast.Node) bool {
		typeSpec, ok := node.(*ast.TypeSpec)
		if !ok {
			return true
		}
		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return true
		}

		fields := make([]jsonFieldContract, 0)
		for _, field := range structType.Fields.List {
			if field.Tag == nil || len(field.Names) == 0 {
				continue
			}
			tag, err := strconvUnquote(field.Tag.Value)
			if err != nil {
				continue
			}
			jsonTag := reflect.StructTag(tag).Get("json")
			if jsonTag == "" || jsonTag == "-" {
				continue
			}
			parts := strings.Split(jsonTag, ",")
			jsonName := parts[0]
			if jsonName == "" {
				jsonName = field.Names[0].Name
			}
			fields = append(fields, jsonFieldContract{
				GoName:    field.Names[0].Name,
				JSONName:  jsonName,
				OmitEmpty: slices.Contains(parts[1:], "omitempty"),
			})
		}
		if len(fields) == 0 {
			return true
		}
		*contracts = append(*contracts, jsonStructContract{Type: pkg + "." + typeSpec.Name.Name, Fields: fields})
		return true
	})
	return nil
}

func strconvUnquote(value string) (string, error) {
	if len(value) < 2 || value[0] != '`' || value[len(value)-1] != '`' {
		return "", fmt.Errorf("not a raw string")
	}
	return value[1 : len(value)-1], nil
}

func jsonFields(value reflect.Type) []jsonFieldContract {
	fields := make([]jsonFieldContract, 0, value.NumField())
	for field := range value.Fields() {
		parts := strings.Split(field.Tag.Get("json"), ",")
		if parts[0] == "" || parts[0] == "-" {
			continue
		}
		fields = append(fields, jsonFieldContract{
			GoName:    field.Name,
			JSONName:  parts[0],
			OmitEmpty: slices.Contains(parts[1:], "omitempty"),
		})
	}
	return fields
}

func repositoryRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("repository root not found")
		}
		dir = parent
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
