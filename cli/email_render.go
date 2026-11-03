package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/spf13/cobra"
)

type emailRenderEntry struct {
	Name    string `json:"name"`
	Subject string `json:"subject"`
	HTML    string `json:"html"`
	Text    string `json:"text"`
	Bytes   int    `json:"bytes"`
}

type emailRenderReport struct {
	Emails []emailRenderEntry `json:"emails"`
}

type emailRenderCommandRunner func(
	rootDir string,
	args []string,
	stdout, stderr io.Writer,
) error

var runEmailRenderCommand emailRenderCommandRunner = func(
	rootDir string,
	args []string,
	stdout, stderr io.Writer,
) error {
	runCmd := exec.Command("go", args...)
	runCmd.Dir = rootDir
	runCmd.Stdin = os.Stdin
	runCmd.Stdout = stdout
	runCmd.Stderr = stderr
	return runCmd.Run()
}

var openEmailGallery = func(indexPath string) error {
	var opener string
	switch runtime.GOOS {
	case "linux":
		opener = "xdg-open"
	case "darwin":
		opener = "open"
	default:
		return output.NewError(
			output.CodeError,
			"opening the email gallery is not supported on "+runtime.GOOS,
			output.ExitUsage,
			"Open "+indexPath+" manually.",
		)
	}

	openCmd := exec.Command(opener, indexPath)
	openCmd.Stdout = io.Discard
	openCmd.Stderr = io.Discard
	if err := openCmd.Start(); err != nil {
		return output.WrapError(
			output.CodeExternalCommandFailed,
			fmt.Errorf("open email gallery: %w", err),
			output.ExitExternal,
			"Open "+indexPath+" manually.",
		)
	}
	return openCmd.Process.Release()
}

func newEmailRenderCommand() *cobra.Command {
	var open bool
	var watch bool
	cmd := &cobra.Command{
		Use:   "render [name]",
		Short: "Render emails to HTML, text and a gallery",
		Long: `Render every email preview through the real Go ToHTML and ToText path with fixture data.

Writes one .html and .txt per email to tmp/emails plus an index.html gallery
with desktop, mobile, dark and text panes. Nothing is sent; use Mailpit or the
application when you need delivery.

Pass an email name to render only that email, --json for a machine-readable
report, --open to open the gallery in the default browser, or --watch to keep
the gallery live while editing emails/.`,
		Example: `  andurel email render
  andurel email render verify_email
  andurel email render --json
  andurel email render --open
  andurel email render --watch --open`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			return runEmailRender(cmd, name, open, watch)
		},
	}
	cmd.Flags().BoolVar(&open, "open", false, "Open tmp/emails/index.html after rendering")
	cmd.Flags().BoolVar(
		&watch,
		"watch",
		false,
		"Re-render when emails/ changes; incompatible with --json",
	)

	return cmd
}

func runEmailRender(cmd *cobra.Command, name string, openGallery, watch bool) error {
	rootDir, err := findGoModRoot()
	if err != nil {
		return err
	}

	cmdEmailsMain := filepath.Join(rootDir, "cmd", "emails", "main.go")
	if _, err := os.Stat(cmdEmailsMain); err != nil {
		if os.IsNotExist(err) {
			return output.NewError(
				output.CodeMissingTool,
				fmt.Sprintf("email preview entrypoint not found at %s", cmdEmailsMain),
				output.ExitDependency,
				"This scaffold is older; copy cmd/emails/main.go and emails/previews.go from a fresh andurel new project.",
			)
		}
		return err
	}

	opts, err := output.ParseOptions(cmd)
	if err != nil {
		return err
	}
	structured := output.UsesStructuredOutput(opts)
	if watch {
		if structured {
			return output.NewError(
				output.CodeOutputMode,
				"--watch cannot be combined with --json",
				output.ExitUsage,
				"Run andurel email render --watch without an output mode flag, or use --json without --watch.",
			)
		}
		return runEmailRenderWatch(cmd, rootDir, name, openGallery)
	}

	goArgs := emailRenderGoArgs(name, structured, false)
	if structured {
		var stdout, stderr bytes.Buffer
		if err := runEmailRenderCommand(rootDir, goArgs, &stdout, &stderr); err != nil {
			return emailRenderCommandError(err, stderr.String())
		}
		if openGallery {
			if err := openEmailGallery(filepath.Join(rootDir, "tmp", "emails", "index.html")); err != nil {
				return err
			}
		}

		var report emailRenderReport
		if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &report); err != nil {
			return output.WrapError(
				output.CodeExternalCommandFailed,
				fmt.Errorf("decode email render report: %w", err),
				output.ExitExternal,
				"Run andurel email render without --json to inspect the raw renderer output.",
			)
		}
		return output.OK(
			cmd,
			report,
			emailRenderSummary(report),
			output.Breadcrumb{
				Command:     "andurel email render --open",
				Description: "open the rendered gallery",
			},
		)
	}

	if err := runEmailRenderCommand(rootDir, goArgs, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
		return emailRenderCommandError(err, "")
	}
	if openGallery {
		return openEmailGallery(filepath.Join(rootDir, "tmp", "emails", "index.html"))
	}
	return nil
}

// emailRenderGoArgs builds the go run arguments shared by the one-shot and
// watch paths. The watch path asks cmd/emails for JSON so it can report the
// email count without parsing the human summary.
func emailRenderGoArgs(name string, jsonReport, watch bool) []string {
	goArgs := []string{"run", "./cmd/emails", "-out", "tmp/emails"}
	if name != "" {
		goArgs = append(goArgs, "-only", name)
	}
	if jsonReport {
		goArgs = append(goArgs, "-json")
	}
	if watch {
		goArgs = append(goArgs, "-watch")
	}
	return goArgs
}

func emailRenderCommandError(err error, stderr string) error {
	hint := strings.TrimSpace(stderr)
	if hint == "" {
		hint = "Inspect the render error above and retry."
	}
	return output.WrapError(
		output.CodeExternalCommandFailed,
		fmt.Errorf("run email preview renderer: %w", err),
		output.ExitExternal,
		hint,
	)
}

func emailRenderSummary(report emailRenderReport) string {
	switch len(report.Emails) {
	case 0:
		return "Rendered no email previews"
	case 1:
		return "Rendered 1 email preview to tmp/emails"
	default:
		return fmt.Sprintf("Rendered %d email previews to tmp/emails", len(report.Emails))
	}
}
