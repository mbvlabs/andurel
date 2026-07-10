package upgrade

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/mbvlabs/andurel/layout"
	"github.com/mbvlabs/andurel/layout/templates"
)

type rcFileTransform struct {
	path              string
	templateName      string
	exactLegacyHashes map[string]struct{}
	replaceFunctions  map[string]map[string]struct{}
	addFunctions      []string
	imports           []string
}

var rcFileTransforms = []rcFileTransform{
	{
		path: "models/user.go", templateName: "models_user.tmpl",
		exactLegacyHashes: hashSet("8590ed3233727963de7536023bf93e93722aa04544ee6ba0b24cb2b067746dea"),
		replaceFunctions: map[string]map[string]struct{}{
			"Update":   hashSet("48ff8e13da4fdafd87aeb16974c71973094abc3eb43e0db99a5c633fe117c8d0"),
			"Destroy":  hashSet("0d4c8cda5e8ecfdd7ab91c9f127c337563769d8e3f43f3e0d25960202b34731a"),
			"Paginate": hashSet("a0884d87c5a45e3ac1a2a52b26554cb6dc39f48fbd1e47b059661a0b63ed934b"),
		},
	},
	{
		path: "router/router.go", templateName: "router_router.tmpl",
		exactLegacyHashes: hashSet(
			"6b8b84bd87e91154e9e0f8f89fbbbc27484530c07113a8a6d0955dcc66a45e52",
		),
		replaceFunctions: map[string]map[string]struct{}{
			"SetupGlobalMiddleware": hashSet(
				"7983213286e7e15cd55121acc5e35b2d43d6545b16b12891d07d4aedaf55c8ef",
				"c973e22df41997fa6825096b7e932bf6e85ebbfb65b334eb799f6b86bf6347af",
			),
		},
		addFunctions: []string{"newApplicationSessionStore", "newCORSConfig"},
		imports:      []string{"fmt", "strings", "{{module}}/internal/server"},
	},
	{
		path: "cmd/app/main.go", templateName: "cmd_app_main.tmpl",
		exactLegacyHashes: hashSet(
			"e3fa9840a2ce057c95f14a32187a68232bcef8141744f746bee80588c2b28992",
		),
		replaceFunctions: map[string]map[string]struct{}{
			"startQueueProcessor": hashSet("d05a1f1fb57e3720a4e20768bdae301aafd7876f320faf42a3630cebff1fd9d3"),
			"startServer":         hashSet("4fb7a65f1b3a67f06f27f3719ce5197c5d330e19706bb5751799d7a956dabbc2"),
		},
		addFunctions: []string{"startInBackground", "stopAndWait"},
		imports:      []string{"errors"},
	},
}

func hashSet(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func transformRCProject(
	projectRoot string,
	generator *TemplateGenerator,
	lock *layout.AndurelLock,
) ([]transformedFile, []string, error) {
	modulePath, err := resolveModulePath(projectRoot)
	if err != nil {
		return nil, nil, err
	}
	data := generator.buildTemplateData(*lock.ScaffoldConfig, modulePath, lock.ExtensionNames())
	changes := make([]transformedFile, 0, len(rcFileTransforms))
	var conflicts []string
	for _, spec := range rcFileTransforms {
		target, renderErr := renderTemplateToBytes(spec.templateName, templates.Files, data)
		if renderErr != nil {
			return nil, nil, renderErr
		}
		path := filepath.Join(projectRoot, spec.path)
		current, readErr := os.ReadFile(path)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				conflicts = append(conflicts, fmt.Sprintf("%s is missing", spec.path))
				continue
			}
			return nil, nil, readErr
		}
		if bytes.Equal(current, target) {
			continue
		}
		if _, recognized := spec.exactLegacyHashes[normalizedFileHash(current, modulePath, lock.ScaffoldConfig.ProjectName)]; recognized {
			changes = append(changes, transformedFile{Path: spec.path, Content: target})
			continue
		}
		transformed, conflict, transformErr := transformEditedRCFile(current, target, modulePath, spec)
		if transformErr != nil {
			return nil, nil, transformErr
		}
		if conflict != "" {
			conflicts = append(conflicts, fmt.Sprintf("%s: %s", spec.path, conflict))
			continue
		}
		if !bytes.Equal(current, transformed) {
			changes = append(changes, transformedFile{Path: spec.path, Content: transformed})
		}
	}
	return changes, conflicts, nil
}

func normalizedFileHash(content []byte, values ...string) string {
	normalized := slices.Clone(content)
	for _, value := range values {
		if value != "" {
			normalized = bytes.ReplaceAll(normalized, []byte(value), []byte("{{.ModuleName}}"))
		}
	}
	digest := sha256.Sum256(normalized)
	return fmt.Sprintf("%x", digest)
}

type textEdit struct {
	start int
	end   int
	text  []byte
}

func transformEditedRCFile(current, target []byte, modulePath string, spec rcFileTransform) ([]byte, string, error) {
	sourceSet := token.NewFileSet()
	sourceFile, err := parser.ParseFile(sourceSet, spec.path, current, parser.ParseComments)
	if err != nil {
		return nil, "source is not valid Go", nil
	}
	targetSet := token.NewFileSet()
	targetFile, err := parser.ParseFile(targetSet, spec.path, target, parser.ParseComments)
	if err != nil {
		return nil, "", fmt.Errorf("parse target %s: %w", spec.path, err)
	}
	sourceFunctions := functionDeclarations(sourceFile)
	targetFunctions := functionDeclarations(targetFile)
	var edits []textEdit

	names := make([]string, 0, len(spec.replaceFunctions))
	for name := range spec.replaceFunctions {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		source := sourceFunctions[name]
		targetDecls := targetFunctions[name]
		if len(source) != 1 {
			return nil, fmt.Sprintf("expected exactly one %s function, found %d", name, len(source)), nil
		}
		if len(targetDecls) != 1 {
			return nil, "", fmt.Errorf("target template has %d %s functions", len(targetDecls), name)
		}
		sourceHash, hashErr := functionHash(sourceSet, source[0])
		if hashErr != nil {
			return nil, "", hashErr
		}
		targetText, targetHash, hashErr := functionBytesAndHash(targetSet, targetDecls[0])
		if hashErr != nil {
			return nil, "", hashErr
		}
		if sourceHash == targetHash {
			continue
		}
		if _, ok := spec.replaceFunctions[name][sourceHash]; !ok {
			return nil, fmt.Sprintf("%s has unrecognized or ambiguous edits", name), nil
		}
		edits = append(edits, textEdit{
			start: sourceSet.Position(source[0].Pos()).Offset,
			end:   sourceSet.Position(source[0].End()).Offset,
			text:  bytes.TrimSuffix(targetText, []byte("\n")),
		})
	}

	for _, name := range spec.addFunctions {
		source := sourceFunctions[name]
		targetDecls := targetFunctions[name]
		if len(targetDecls) != 1 {
			return nil, "", fmt.Errorf("target template has %d %s functions", len(targetDecls), name)
		}
		if len(source) > 1 {
			return nil, fmt.Sprintf("expected at most one %s function, found %d", name, len(source)), nil
		}
		targetText, targetHash, hashErr := functionBytesAndHash(targetSet, targetDecls[0])
		if hashErr != nil {
			return nil, "", hashErr
		}
		if len(source) == 1 {
			sourceHash, sourceErr := functionHash(sourceSet, source[0])
			if sourceErr != nil {
				return nil, "", sourceErr
			}
			if sourceHash != targetHash {
				return nil, fmt.Sprintf("%s already exists with unrecognized content", name), nil
			}
			continue
		}
		edits = append(edits, textEdit{start: len(current), end: len(current), text: append([]byte("\n"), targetText...)})
	}

	sort.SliceStable(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	result := append([]byte(nil), current...)
	for _, edit := range edits {
		result = append(result[:edit.start], append(edit.text, result[edit.end:]...)...)
	}
	formatted, err := format.Source(result)
	if err != nil {
		return nil, "", fmt.Errorf("format transformed %s: %w", spec.path, err)
	}
	formatted, err = addRequiredImports(formatted, modulePath, spec.imports)
	if err != nil {
		return nil, "", err
	}
	return formatted, "", nil
}

func functionDeclarations(file *ast.File) map[string][]*ast.FuncDecl {
	result := make(map[string][]*ast.FuncDecl)
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok {
			result[function.Name.Name] = append(result[function.Name.Name], function)
		}
	}
	return result
}

func functionHash(fset *token.FileSet, function *ast.FuncDecl) (string, error) {
	_, hash, err := functionBytesAndHash(fset, function)
	return hash, err
}

func functionBytesAndHash(fset *token.FileSet, function *ast.FuncDecl) ([]byte, string, error) {
	var buffer bytes.Buffer
	if err := format.Node(&buffer, fset, function); err != nil {
		return nil, "", err
	}
	buffer.WriteByte('\n')
	digest := sha256.Sum256(buffer.Bytes())
	return buffer.Bytes(), fmt.Sprintf("%x", digest), nil
}

func addRequiredImports(content []byte, modulePath string, required []string) ([]byte, error) {
	if len(required) == 0 {
		return content, nil
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "transformed.go", content, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	existing := make(map[string]struct{})
	for _, spec := range file.Imports {
		path, unquoteErr := strconv.Unquote(spec.Path.Value)
		if unquoteErr != nil {
			return nil, unquoteErr
		}
		existing[path] = struct{}{}
	}
	var importDecl *ast.GenDecl
	for _, declaration := range file.Decls {
		gen, ok := declaration.(*ast.GenDecl)
		if ok && gen.Tok == token.IMPORT {
			importDecl = gen
			break
		}
	}
	if importDecl == nil {
		return nil, fmt.Errorf("transformed file has no import declaration")
	}
	for _, path := range required {
		path = strings.ReplaceAll(path, "{{module}}", modulePath)
		if _, ok := existing[path]; ok {
			continue
		}
		importDecl.Specs = append(importDecl.Specs, &ast.ImportSpec{Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(path)}})
		existing[path] = struct{}{}
	}
	var buffer bytes.Buffer
	if err := format.Node(&buffer, fset, file); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
