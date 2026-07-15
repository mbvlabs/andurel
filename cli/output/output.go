// Package output defines the shared CLI response contract for agent-friendly
// output modes.
package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const (
	// CodeError is the generic structured error code.
	CodeError = "error"
	// CodeUsage identifies invalid command usage.
	CodeUsage = "usage_error"
	// CodeOutputMode identifies an invalid output mode selection.
	CodeOutputMode = "invalid_output_mode"
	// CodeProjectNotFound identifies commands run outside an Andurel project.
	CodeProjectNotFound = "project_not_found"
	// CodeMissingTool identifies a missing external tool dependency.
	CodeMissingTool = "missing_tool"
	// CodeInvalidExtension identifies an unknown extension name.
	CodeInvalidExtension = "invalid_extension"
	// CodeInvalidInertiaAdapter identifies an unsupported Inertia adapter.
	CodeInvalidInertiaAdapter = "invalid_inertia_adapter"
	// CodeUnsafeAction identifies an action that requires explicit confirmation.
	CodeUnsafeAction = "unsafe_action_requires_confirmation"
	// CodeGenerationFailed identifies a failed code generation command.
	CodeGenerationFailed = "generation_failed"
	// CodeExternalCommandFailed identifies a failed external command.
	CodeExternalCommandFailed = "external_command_failed"
	// CodeConfigError identifies invalid or unreadable configuration.
	CodeConfigError = "config_error"
	// CodeAmbiguousInput identifies user input that cannot be resolved safely.
	CodeAmbiguousInput = "ambiguous_input"
	// CodeUpdateRequired identifies an operation that requires a newer Andurel CLI.
	CodeUpdateRequired = "update_required"
	// ExitUsage is the exit code for command usage errors.
	ExitUsage = 1
	// ExitProject is the exit code for missing project context.
	ExitProject = 2
	// ExitDependency is the exit code for missing dependencies.
	ExitDependency = 3
	// ExitUnsafe is the exit code for unsafe unconfirmed actions.
	ExitUnsafe = 4
	// ExitGeneration is the exit code for generation failures.
	ExitGeneration = 5
	// ExitExternal is the exit code for failed external commands.
	ExitExternal = 6
	// ExitConfig is the exit code for configuration errors.
	ExitConfig = 7
	// ExitAmbiguous is the exit code for ambiguous input.
	ExitAmbiguous = 8
)

// Mode is the selected output format for a command invocation.
type Mode string

const (
	// ModeHuman renders the default human-readable output.
	ModeHuman Mode = "human"
	// ModeJSON renders structured JSON output.
	ModeJSON Mode = "json"
	// ModeAgent renders structured output optimized for agent workflows.
	ModeAgent Mode = "agent"
	// ModeMarkdown renders Markdown output.
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

// Error returns the best available human-readable error message.
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

// Unwrap returns the underlying cause, if one was recorded.
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
	flags.String("jq", "", "Select a simple field path from command data and emit JSON directly")
	flags.Bool("ids-only", false, "Emit one resource identifier per line when supported")
	flags.Bool("count", false, "Emit one raw resource count when supported")
	flags.Bool("verbose", false, "Emit verbose output")
}

// ParseOptions returns output options from command flags.
func ParseOptions(cmd *cobra.Command) (Options, error) {
	return parseOptions(cmd, true)
}

func parseOptions(cmd *cobra.Command, validateProjections bool) (Options, error) {
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

	projections := 0
	for _, selected := range []bool{opts.JQ != "", opts.IDsOnly, opts.Count} {
		if selected {
			projections++
		}
	}
	if validateProjections && projections > 1 {
		return opts, NewError(
			CodeOutputMode,
			"choose only one projection flag: --jq, --ids-only, or --count",
			ExitUsage,
			"Run the command again with a single projection flag.",
		)
	}
	if projections > 0 && opts.Mode == ModeHuman {
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

	envelope := Envelope{
		OK:          true,
		Data:        data,
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
		return WrapError(CodeInvalidInertiaAdapter, err, ExitUsage, "Use vue, react, or svelte, optionally followed by /npm, /pnpm, /bun, or /yarn.")
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
		opts, _ = parseOptions(cmd, false)
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
	switch {
	case opts.JQ != "":
		selected, err := applyJQ(envelope.Data, opts.JQ)
		if err != nil {
			return err
		}
		return writeJSON(w, selected)
	case opts.IDsOnly:
		return writeIDs(w, envelope.Data)
	case opts.Count:
		return writeCount(w, envelope.Data)
	}

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
	normalized, err := normalizeJSONValue(data)
	if err != nil {
		return nil, NewError(CodeUsage, "command data cannot be projected as JSON", ExitUsage, err.Error())
	}
	if expr == "" || expr == "." {
		return normalized, nil
	}
	if !strings.HasPrefix(expr, ".") {
		return nil, NewError(CodeUsage, "only simple jq-style field paths are supported", ExitUsage, "Use an expression like .field or .nested.field.")
	}

	current := normalized
	parts := strings.SplitSeq(strings.TrimPrefix(expr, "."), ".")
	for part := range parts {
		if part == "" {
			continue
		}
		object, ok := current.(map[string]any)
		if !ok {
			return nil, NewError(CodeUsage, "jq path not found: "+expr, ExitUsage, "The selected value is not an object with field "+part+".")
		}
		next, ok := object[part]
		if !ok {
			return nil, NewError(CodeUsage, "jq path not found: "+expr, ExitUsage, "Use andurel commands --json to inspect available fields.")
		}
		current = next
	}
	return current, nil
}

func normalizeJSONValue(value any) (any, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var normalized any
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		return nil, err
	}
	return normalized, nil
}

func writeIDs(w io.Writer, data any) error {
	items, err := projectionItems(data)
	if err != nil {
		return err
	}
	for _, item := range items {
		identifier, ok := projectionIdentifier(item)
		if !ok {
			return NewError(CodeUsage, "command data does not contain projectable identifiers", ExitUsage, "Use --jq to select a specific identifier field.")
		}
		if _, err := fmt.Fprintln(w, identifier); err != nil {
			return err
		}
	}
	return nil
}

func writeCount(w io.Writer, data any) error {
	items, err := projectionItems(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, len(items))
	return err
}

func projectionItems(data any) ([]any, error) {
	normalized, err := normalizeJSONValue(data)
	if err != nil {
		return nil, NewError(CodeUsage, "command data cannot be projected", ExitUsage, err.Error())
	}
	if items, ok := normalized.([]any); ok {
		return items, nil
	}
	object, ok := normalized.(map[string]any)
	if !ok {
		return nil, NewError(CodeUsage, "command data is not a projectable collection", ExitUsage, "Use --jq for scalar or object data.")
	}
	for _, field := range []string{"routes", "items", "results", "names", "extensions", "tools"} {
		if items, ok := object[field].([]any); ok {
			return items, nil
		}
	}
	return nil, NewError(CodeUsage, "command data is not a projectable collection", ExitUsage, "Use --jq to select a collection field.")
}

func projectionIdentifier(value any) (string, bool) {
	switch value := value.(type) {
	case string:
		return value, true
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64), true
	case map[string]any:
		for _, field := range []string{"id", "name", "variable", "path"} {
			if identifier, ok := value[field].(string); ok && identifier != "" {
				return identifier, true
			}
		}
	}
	return "", false
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
