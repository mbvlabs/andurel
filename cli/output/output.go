// Package output defines the shared CLI response contract for agent-friendly
// output modes.
package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const (
	CodeError                 = "error"
	CodeUsage                 = "usage_error"
	CodeOutputMode            = "invalid_output_mode"
	CodeProjectNotFound       = "project_not_found"
	CodeMissingTool           = "missing_tool"
	CodeInvalidExtension      = "invalid_extension"
	CodeInvalidInertiaAdapter = "invalid_inertia_adapter"
	CodeUnsafeAction          = "unsafe_action_requires_confirmation"
	CodeGenerationFailed      = "generation_failed"
	CodeExternalCommandFailed = "external_command_failed"
	CodeConfigError           = "config_error"
	CodeAmbiguousInput        = "ambiguous_input"
	ExitUsage                 = 1
	ExitProject               = 2
	ExitDependency            = 3
	ExitUnsafe                = 4
	ExitGeneration            = 5
	ExitExternal              = 6
	ExitConfig                = 7
	ExitAmbiguous             = 8
)

// Mode is the selected output format for a command invocation.
type Mode string

const (
	ModeHuman    Mode = "human"
	ModeJSON     Mode = "json"
	ModeAgent    Mode = "agent"
	ModeMarkdown Mode = "markdown"
)

// Options are parsed from the root persistent output flags.
type Options struct {
	Mode    Mode
	Quiet   bool
	JQ      string
	IDsOnly bool
	Count   bool
	Verbose bool
}

// Breadcrumb is a follow-up action an agent or human can take.
type Breadcrumb struct {
	Command     string `json:"cmd"`
	Description string `json:"description,omitempty"`
}

// Envelope wraps successful structured command output.
type Envelope struct {
	OK          bool         `json:"ok"`
	Data        any          `json:"data,omitempty"`
	Summary     string       `json:"summary,omitempty"`
	Breadcrumbs []Breadcrumb `json:"breadcrumbs,omitempty"`
}

// ErrorEnvelope wraps failed structured command output.
type ErrorEnvelope struct {
	OK       bool   `json:"ok"`
	Code     string `json:"code"`
	Error    string `json:"error"`
	Hint     string `json:"hint,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
}

// CLIError is a typed command error with a stable machine-readable code.
type CLIError struct {
	Code     string
	Message  string
	Hint     string
	ExitCode int
	Cause    error
}

func (e *CLIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return e.Code
}

func (e *CLIError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// NewError creates a typed CLI error.
func NewError(code, message string, exitCode int, hint string) *CLIError {
	return &CLIError{
		Code:     code,
		Message:  message,
		Hint:     hint,
		ExitCode: exitCode,
	}
}

// WrapError creates a typed CLI error while preserving the wrapped cause.
func WrapError(code string, cause error, exitCode int, hint string) *CLIError {
	return &CLIError{
		Code:     code,
		Cause:    cause,
		ExitCode: exitCode,
		Hint:     hint,
	}
}

// ExitCode returns the process exit code for an error.
func ExitCode(err error) int {
	envelope := Fail(err)
	if envelope.ExitCode == 0 {
		return ExitUsage
	}
	return envelope.ExitCode
}

// RegisterPersistentFlags adds the shared agent-friendly output flags.
func RegisterPersistentFlags(cmd *cobra.Command) {
	flags := cmd.PersistentFlags()
	flags.Bool("json", false, "Emit JSON output")
	flags.Bool("agent", false, "Emit structured output optimized for agents")
	flags.Bool("md", false, "Emit Markdown output")
	flags.Bool("quiet", false, "Suppress non-essential human output")
	flags.String("jq", "", "Apply a jq expression to structured output")
	flags.Bool("ids-only", false, "Emit only resource identifiers when supported")
	flags.Bool("count", false, "Emit only resource counts when supported")
	flags.Bool("verbose", false, "Emit verbose output")
}

// ParseOptions returns output options from command flags.
func ParseOptions(cmd *cobra.Command) (Options, error) {
	opts := Options{Mode: ModeHuman}
	if cmd == nil {
		return opts, nil
	}

	jsonMode, _ := boolFlag(cmd, "json")
	agentMode, _ := boolFlag(cmd, "agent")
	mdMode, _ := boolFlag(cmd, "md")

	selected := 0
	for _, enabled := range []bool{jsonMode, agentMode, mdMode} {
		if enabled {
			selected++
		}
	}
	if selected > 1 {
		return opts, NewError(
			CodeOutputMode,
			"choose only one output mode: --json, --agent, or --md",
			ExitUsage,
			"Run the command again with a single output mode flag.",
		)
	}

	switch {
	case agentMode:
		opts.Mode = ModeAgent
	case jsonMode:
		opts.Mode = ModeJSON
	case mdMode:
		opts.Mode = ModeMarkdown
	}

	opts.Quiet, _ = boolFlag(cmd, "quiet")
	opts.JQ, _ = stringFlag(cmd, "jq")
	opts.IDsOnly, _ = boolFlag(cmd, "ids-only")
	opts.Count, _ = boolFlag(cmd, "count")
	opts.Verbose, _ = boolFlag(cmd, "verbose")
	if opts.JQ != "" && opts.Mode == ModeHuman {
		opts.Mode = ModeJSON
	}

	return opts, nil
}

func boolFlag(cmd *cobra.Command, name string) (bool, error) {
	flag := cmd.Flag(name)
	if flag == nil {
		return false, nil
	}
	return strconv.ParseBool(flag.Value.String())
}

func stringFlag(cmd *cobra.Command, name string) (string, error) {
	flag := cmd.Flag(name)
	if flag == nil {
		return "", nil
	}
	return flag.Value.String(), nil
}

// UsesStructuredOutput reports whether a command should render through the
// shared output contract instead of human prose.
func UsesStructuredOutput(opts Options) bool {
	return opts.Mode != ModeHuman
}

// SuppressesHumanOutput reports whether command progress from lower-level
// routines should be hidden before rendering the final response.
func SuppressesHumanOutput(opts Options) bool {
	return UsesStructuredOutput(opts) || opts.Quiet
}

// OK renders a successful response using the selected output mode.
func OK(cmd *cobra.Command, data any, summary string, breadcrumbs ...Breadcrumb) error {
	opts, err := ParseOptions(cmd)
	if err != nil {
		return err
	}

	renderData := data
	if opts.JQ != "" {
		renderData, err = applyJQ(data, opts.JQ)
		if err != nil {
			return err
		}
	}

	envelope := Envelope{
		OK:          true,
		Data:        renderData,
		Summary:     summary,
		Breadcrumbs: breadcrumbs,
	}

	return renderOK(cmd.OutOrStdout(), opts, envelope)
}

// Fail converts any error into a structured error envelope.
func Fail(err error) ErrorEnvelope {
	if err == nil {
		return ErrorEnvelope{OK: false, Code: CodeError, Error: "unknown error", ExitCode: ExitUsage}
	}

	var cliErr *CLIError
	if errors.As(err, &cliErr) {
		code := cliErr.Code
		if code == "" {
			code = CodeError
		}
		exitCode := cliErr.ExitCode
		if exitCode == 0 {
			exitCode = ExitUsage
		}
		return ErrorEnvelope{
			OK:       false,
			Code:     code,
			Error:    cliErr.Error(),
			Hint:     cliErr.Hint,
			ExitCode: exitCode,
		}
	}

	classified := classifyError(err)
	if classified != nil {
		return Fail(classified)
	}

	return ErrorEnvelope{
		OK:       false,
		Code:     CodeError,
		Error:    err.Error(),
		ExitCode: ExitUsage,
	}
}

func classifyError(err error) *CLIError {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "not in an andurel project") ||
		strings.Contains(msg, "go.mod could not be found") ||
		strings.Contains(msg, "go.mod not found"):
		return WrapError(
			CodeProjectNotFound,
			err,
			ExitProject,
			"Run this from a directory containing an Andurel project's go.mod file.",
		)
	case strings.Contains(msg, "bin/") && strings.Contains(msg, "not found"):
		return WrapError(CodeMissingTool, err, ExitDependency, "Run andurel tool sync to install project tools.")
	case strings.Contains(msg, "unknown extension") || strings.Contains(msg, "invalid extension"):
		return WrapError(CodeInvalidExtension, err, ExitUsage, "Run andurel extension list --available to inspect available extensions.")
	case strings.Contains(msg, "invalid inertia adapter"):
		return WrapError(CodeInvalidInertiaAdapter, err, ExitUsage, "Use vue or react, optionally followed by /npm, /pnpm, /bun, or /yarn.")
	case strings.Contains(msg, "requires --force") || strings.Contains(msg, "use --force") || strings.Contains(msg, "without --force"):
		return WrapError(CodeUnsafeAction, err, ExitUnsafe, "Re-run with --force after confirming the destructive action is intended.")
	case strings.Contains(msg, "generation failed") || strings.Contains(msg, "failed to generate"):
		return WrapError(CodeGenerationFailed, err, ExitGeneration, "Inspect the error details and generated files, then retry.")
	case strings.Contains(msg, "external command") || strings.Contains(msg, "command failed"):
		return WrapError(CodeExternalCommandFailed, err, ExitExternal, "Inspect the command output and required tools.")
	case strings.Contains(msg, "lock file") || strings.Contains(msg, "andurel.lock") || strings.Contains(msg, "config"):
		return WrapError(CodeConfigError, err, ExitConfig, "Inspect andurel.lock and .andurel/config.json.")
	case strings.Contains(msg, "ambiguous"):
		return WrapError(CodeAmbiguousInput, err, ExitAmbiguous, "Provide a more specific command argument.")
	default:
		return nil
	}
}

// RenderError renders a failed response using the selected output mode.
func RenderError(cmd *cobra.Command, err error) error {
	opts, parseErr := ParseOptions(cmd)
	if parseErr != nil {
		err = parseErr
	}

	envelope := Fail(err)
	if opts.Mode == ModeJSON || opts.Mode == ModeAgent {
		return writeJSON(cmd.ErrOrStderr(), envelope)
	}
	if opts.Mode == ModeMarkdown {
		if envelope.Hint != "" {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "**Error:** %s\n\n%s\n", envelope.Error, envelope.Hint)
			return nil
		}
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "**Error:** %s\n", envelope.Error)
		return nil
	}

	if envelope.Hint != "" {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s\nHint: %s\n", envelope.Error, envelope.Hint)
		return nil
	}
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s\n", envelope.Error)
	return nil
}

func renderOK(w io.Writer, opts Options, envelope Envelope) error {
	switch opts.Mode {
	case ModeJSON, ModeAgent:
		return writeJSON(w, envelope)
	case ModeMarkdown:
		return writeMarkdown(w, opts, envelope)
	default:
		return writeHuman(w, opts, envelope)
	}
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func applyJQ(data any, expr string) (any, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" || expr == "." {
		return data, nil
	}
	if !strings.HasPrefix(expr, ".") {
		return nil, NewError(CodeUsage, "only simple jq-style field paths are supported", ExitUsage, "Use an expression like .field or .nested.field.")
	}

	current := any(map[string]any{"data": data})
	parts := strings.SplitSeq(strings.TrimPrefix(expr, "."), ".")
	for part := range parts {
		if part == "" {
			continue
		}
		next, ok := lookupField(current, part)
		if !ok {
			return nil, NewError(CodeUsage, "jq path not found: "+expr, ExitUsage, "Use andurel commands --json to inspect available fields.")
		}
		current = next
	}
	return current, nil
}

func lookupField(value any, name string) (any, bool) {
	if value == nil {
		return nil, false
	}
	if m, ok := value.(map[string]any); ok {
		v, exists := m[name]
		return v, exists
	}

	rv := reflect.ValueOf(value)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, false
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil, false
	}

	rt := rv.Type()
	for i := range rv.NumField() {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}
		jsonName := strings.Split(field.Tag.Get("json"), ",")[0]
		if jsonName == "" {
			jsonName = field.Name
		}
		if jsonName == name || strings.EqualFold(field.Name, name) {
			return rv.Field(i).Interface(), true
		}
	}
	return nil, false
}

func writeMarkdown(w io.Writer, opts Options, envelope Envelope) error {
	if envelope.Summary != "" && !opts.Quiet {
		if _, err := fmt.Fprintf(w, "%s\n", envelope.Summary); err != nil {
			return err
		}
	}
	if len(envelope.Breadcrumbs) == 0 || opts.Quiet {
		return nil
	}

	if envelope.Summary != "" {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w, "Next steps:"); err != nil {
		return err
	}
	for _, breadcrumb := range envelope.Breadcrumbs {
		line := breadcrumb.Command
		if breadcrumb.Description != "" {
			line = fmt.Sprintf("%s - %s", line, breadcrumb.Description)
		}
		if _, err := fmt.Fprintf(w, "- `%s`\n", line); err != nil {
			return err
		}
	}
	return nil
}

func writeHuman(w io.Writer, opts Options, envelope Envelope) error {
	if opts.Quiet {
		return nil
	}
	if envelope.Summary != "" {
		if _, err := fmt.Fprintln(w, envelope.Summary); err != nil {
			return err
		}
	}
	if len(envelope.Breadcrumbs) == 0 {
		return nil
	}

	if envelope.Summary != "" {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w, "Next steps:"); err != nil {
		return err
	}
	for _, breadcrumb := range envelope.Breadcrumbs {
		line := strings.TrimSpace(breadcrumb.Command)
		if breadcrumb.Description != "" {
			line = fmt.Sprintf("%s - %s", line, breadcrumb.Description)
		}
		if _, err := fmt.Fprintf(w, "  %s\n", line); err != nil {
			return err
		}
	}
	return nil
}
