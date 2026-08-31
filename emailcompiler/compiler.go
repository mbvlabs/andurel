// Package emailcompiler compiles Tailwind-authored Templ email templates into
// Go code that renders email-compatible inline styles.
package emailcompiler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/a-h/templ/generator"
	"github.com/a-h/templ/parser/v2"
	"github.com/a-h/templ/parser/v2/visitor"
)

// Config controls one email compilation pass.
type Config struct {
	ProjectRoot  string
	EmailDir     string
	CSSInputPath string
	TailwindPath string
}

type sourceFile struct {
	path     string
	relative string
	template *parser.TemplateFile
}

type classUse struct {
	name string
	file string
	line uint32
	col  uint32
}

const (
	headCSSStartMarker = "/* andurel:head:start */"
	headCSSEndMarker   = "/* andurel:head:end */"
)

// Compile transforms every .templ file below the configured email directory
// in memory and writes the resulting _templ.go files. Authored templates are
// never changed.
func Compile(ctx context.Context, cfg Config) error {
	resolved, err := resolveConfig(cfg)
	if err != nil {
		return err
	}

	files, err := parseEmailTemplates(resolved.ProjectRoot, resolved.EmailDir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no email templates found in %s", resolved.EmailDir)
	}

	uses, err := collectClassUses(files)
	if err != nil {
		return err
	}

	stylesheet, headCSS, err := compileTailwind(ctx, resolved, uses)
	if err != nil {
		return err
	}

	compiled, err := parseStylesheet(stylesheet)
	if err != nil {
		return fmt.Errorf("parse Tailwind email output: %w", err)
	}

	residual := newResidualStylesheet()
	for _, file := range files {
		if err := transformTemplate(file, compiled, residual); err != nil {
			return err
		}
	}

	retainedCSS := strings.TrimSpace(strings.Join([]string{headCSS, residual.String()}, "\n"))
	if retainedCSS != "" {
		if !injectHeadStyles(files, retainedCSS) {
			return errors.New(
				"retained email styles require a <head> element in the email templates",
			)
		}
	}

	for _, file := range files {
		if err := generateTemplate(file); err != nil {
			return err
		}
	}

	return nil
}

func resolveConfig(cfg Config) (Config, error) {
	if cfg.ProjectRoot == "" {
		return Config{}, errors.New("project root is required")
	}
	root, err := filepath.Abs(cfg.ProjectRoot)
	if err != nil {
		return Config{}, fmt.Errorf("resolve project root: %w", err)
	}
	cfg.ProjectRoot = root
	if cfg.EmailDir == "" {
		cfg.EmailDir = filepath.Join(root, "email")
	} else if !filepath.IsAbs(cfg.EmailDir) {
		cfg.EmailDir = filepath.Join(root, cfg.EmailDir)
	}
	if cfg.CSSInputPath == "" {
		cfg.CSSInputPath = filepath.Join(root, "css", "email.css")
	} else if !filepath.IsAbs(cfg.CSSInputPath) {
		cfg.CSSInputPath = filepath.Join(root, cfg.CSSInputPath)
	}
	if cfg.TailwindPath == "" {
		cfg.TailwindPath = filepath.Join(root, "bin", "tailwindcli")
	} else if !filepath.IsAbs(cfg.TailwindPath) {
		cfg.TailwindPath = filepath.Join(root, cfg.TailwindPath)
	}

	for label, path := range map[string]string{
		"email directory": cfg.EmailDir,
		"email CSS input": cfg.CSSInputPath,
		"Tailwind CLI":    cfg.TailwindPath,
	} {
		if _, err := os.Stat(path); err != nil {
			return Config{}, fmt.Errorf("%s not found at %s: %w", label, path, err)
		}
	}

	return cfg, nil
}

func parseEmailTemplates(projectRoot, emailDir string) ([]sourceFile, error) {
	var paths []string
	err := filepath.WalkDir(emailDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".templ" {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan email templates: %w", err)
	}
	sort.Strings(paths)

	files := make([]sourceFile, 0, len(paths))
	for _, path := range paths {
		template, err := parser.Parse(path)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		relative, err := filepath.Rel(projectRoot, path)
		if err != nil {
			return nil, fmt.Errorf("resolve template path %s: %w", path, err)
		}
		files = append(files, sourceFile{
			path:     path,
			relative: filepath.ToSlash(relative),
			template: template,
		})
	}
	return files, nil
}

func collectClassUses(files []sourceFile) ([]classUse, error) {
	var uses []classUse
	for _, file := range files {
		walk := visitor.New()
		walk.ConstantAttribute = func(attr *parser.ConstantAttribute) error {
			if !attributeKeyEqual(attr.Key, "class") {
				return nil
			}
			for className := range strings.FieldsSeq(attr.Value) {
				uses = append(uses, classUse{
					name: className,
					file: file.relative,
					line: attr.ValueRange.From.Line,
					col:  attr.ValueRange.From.Col,
				})
			}
			return nil
		}
		walk.ExpressionAttribute = func(attr *parser.ExpressionAttribute) error {
			if attributeKeyEqual(attr.Key, "class") {
				return compilerError(
					file.relative,
					attr.Range.From,
					"dynamic class attributes are not supported in email templates; use literal classes",
				)
			}
			return nil
		}
		visitConditionalAttribute := walk.ConditionalAttribute
		walk.ConditionalAttribute = func(attr *parser.ConditionalAttribute) error {
			if attributesContainClass(attr.Then) || attributesContainClass(attr.Else) {
				return compilerError(
					file.relative,
					attr.Range.From,
					"conditional class attributes are not supported in email templates; use literal classes",
				)
			}
			return visitConditionalAttribute(attr)
		}
		if err := file.template.Visit(walk); err != nil {
			return nil, err
		}
	}
	return uses, nil
}

func attributesContainClass(attributes []parser.Attribute) bool {
	for _, attribute := range attributes {
		switch attr := attribute.(type) {
		case *parser.ConstantAttribute:
			if attributeKeyEqual(attr.Key, "class") {
				return true
			}
		case *parser.ExpressionAttribute:
			if attributeKeyEqual(attr.Key, "class") {
				return true
			}
		case *parser.ConditionalAttribute:
			if attributesContainClass(attr.Then) || attributesContainClass(attr.Else) {
				return true
			}
		}
	}
	return false
}

func compileTailwind(ctx context.Context, cfg Config, uses []classUse) (string, string, error) {
	input, err := os.ReadFile(cfg.CSSInputPath)
	if err != nil {
		return "", "", fmt.Errorf("read email CSS input: %w", err)
	}
	headCSS, err := extractHeadCSS(string(input))
	if err != nil {
		return "", "", fmt.Errorf("read retained email CSS: %w", err)
	}

	baseClasses := make(map[string]struct{})
	for _, use := range uses {
		_, base, err := splitVariants(use.name)
		if err != nil {
			return "", "", fmt.Errorf("%s:%d:%d: %w", use.file, use.line, use.col, err)
		}
		baseClasses[base] = struct{}{}
	}
	candidates := make([]string, 0, len(baseClasses))
	for candidate := range baseClasses {
		candidates = append(candidates, candidate)
	}
	sort.Strings(candidates)

	cssDir := filepath.Dir(cfg.CSSInputPath)
	temporaryInput, err := os.CreateTemp(cssDir, ".andurel-email-*.css")
	if err != nil {
		return "", "", fmt.Errorf("create temporary email CSS input: %w", err)
	}
	temporaryInputPath := temporaryInput.Name()
	defer os.Remove(temporaryInputPath)

	if _, err := temporaryInput.Write(input); err != nil {
		_ = temporaryInput.Close()
		return "", "", fmt.Errorf("write temporary email CSS input: %w", err)
	}
	if len(candidates) > 0 {
		if _, err := fmt.Fprintf(
			temporaryInput,
			"\n@source inline(%q);\n",
			strings.Join(candidates, " "),
		); err != nil {
			_ = temporaryInput.Close()
			return "", "", fmt.Errorf("write Tailwind email candidates: %w", err)
		}
	}
	if err := temporaryInput.Close(); err != nil {
		return "", "", fmt.Errorf("close temporary email CSS input: %w", err)
	}

	temporaryOutput, err := os.CreateTemp("", "andurel-email-*.css")
	if err != nil {
		return "", "", fmt.Errorf("create temporary Tailwind output: %w", err)
	}
	temporaryOutputPath := temporaryOutput.Name()
	if err := temporaryOutput.Close(); err != nil {
		return "", "", fmt.Errorf("close temporary Tailwind output: %w", err)
	}
	defer os.Remove(temporaryOutputPath)

	command := exec.CommandContext(
		ctx,
		cfg.TailwindPath,
		"-i",
		temporaryInputPath,
		"-o",
		temporaryOutputPath,
	)
	command.Dir = cfg.ProjectRoot
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return "", "", fmt.Errorf(
			"compile email Tailwind CSS: %w: %s",
			err,
			strings.TrimSpace(stderr.String()),
		)
	}

	output, err := os.ReadFile(temporaryOutputPath)
	if err != nil {
		return "", "", fmt.Errorf("read compiled email Tailwind CSS: %w", err)
	}
	return string(output), headCSS, nil
}

func extractHeadCSS(input string) (string, error) {
	start := strings.Index(input, headCSSStartMarker)
	end := strings.Index(input, headCSSEndMarker)
	switch {
	case start < 0 && end < 0:
		return "", nil
	case start < 0 || end < 0 || end < start:
		return "", errors.New("email head CSS markers must be present once and in order")
	}
	contentStart := start + len(headCSSStartMarker)
	return strings.TrimSpace(input[contentStart:end]), nil
}

func generateTemplate(file sourceFile) error {
	var generated bytes.Buffer
	_, err := generator.Generate(
		file.template,
		&generated,
		generator.WithFileName(file.relative),
	)
	if err != nil {
		return fmt.Errorf("generate %s: %w", file.relative, err)
	}
	formatted, err := format.Source(generated.Bytes())
	if err != nil {
		return fmt.Errorf("format generated %s: %w", file.relative, err)
	}
	target := strings.TrimSuffix(file.path, ".templ") + "_templ.go"
	if err := writeFileAtomically(target, formatted, 0o644); err != nil {
		return fmt.Errorf("write generated %s: %w", file.relative, err)
	}
	return nil
}

func writeFileAtomically(path string, content []byte, mode fs.FileMode) (err error) {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".andurel-email-generated-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		removeErr := os.Remove(temporaryPath)
		if removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
			err = errors.Join(err, removeErr)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func attributeKeyEqual(key parser.AttributeKey, expected string) bool {
	constant, ok := key.(parser.ConstantAttributeKey)
	return ok && strings.EqualFold(constant.Name, expected)
}

func compilerError(file string, position parser.Position, message string) error {
	return fmt.Errorf("%s:%d:%d: %s", file, position.Line, position.Col, message)
}
