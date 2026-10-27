package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/mbvlabs/andurel/v2/skills"
	"github.com/spf13/cobra"
)

type skillInstallation struct {
	Harness string `json:"harness"`
	Path    string `json:"path"`
}

type skillReport struct {
	Name          string              `json:"name"`
	Reference     string              `json:"reference,omitempty"`
	Path          string              `json:"path,omitempty"`
	Installations []skillInstallation `json:"installations,omitempty"`
	Body          string              `json:"body,omitempty"`
}

type skillHarness struct {
	name      string
	label     string
	directory string
}

var skillHarnesses = []skillHarness{
	{name: "codex", label: "Codex", directory: ".codex/skills/andurel"},
	{name: "claude", label: "Claude", directory: ".claude/skills/andurel"},
	{name: "pi", label: "Pi", directory: ".pi/skills/andurel"},
	{name: "opencode", label: "OpenCode", directory: ".opencode/skills/andurel"},
	{name: "crush", label: "Crush", directory: ".crush/skills/andurel"},
}

func newSkillCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Arm agents with the embedded Andurel skill",
		Long:  "Show or install the embedded Andurel skill so agents can operate the project.",
	}
	setAgentMetadata(cmd, "skill", "Provides the embedded Andurel agent skill.")

	cmd.AddCommand(&cobra.Command{
		Use:   "show [reference]",
		Short: "Show the embedded Andurel skill or one of its references",
		Long: `Print the embedded Andurel skill.

With a reference name, print that reference file instead of SKILL.md. Accepts
the name ("http-and-routing"), the file path ("references/http-and-routing.md"),
or the same with the .md suffix omitted. Available references:
cli-catalog, http-and-routing, layer-placement.`,
		Example: `  andurel skill show
  andurel skill show layer-placement
  andurel skill show references/http-and-routing.md --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := output.ParseOptions(cmd)
			if err != nil {
				return err
			}

			reference := ""
			body := skills.AndurelSkill
			if len(args) == 1 {
				reference, body, err = loadSkillReference(args[0])
				if err != nil {
					return err
				}
			}

			if opts.Mode == output.ModeJSON || opts.Mode == output.ModeAgent {
				return output.OK(
					cmd,
					skillReport{Name: "andurel", Reference: reference, Body: body},
					"Loaded Andurel skill",
				)
			}
			if opts.Quiet {
				return nil
			}
			if _, err := fmt.Fprint(cmd.OutOrStdout(), body); err != nil {
				return err
			}
			if !strings.HasSuffix(body, "\n") {
				_, err = fmt.Fprintln(cmd.OutOrStdout())
				return err
			}
			return nil
		},
	})

	var requestedHarnesses []string
	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install the embedded Andurel skill into the current project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			harnesses, err := selectSkillHarnesses(cmd, requestedHarnesses)
			if err != nil {
				return err
			}
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}

			installations := make([]skillInstallation, 0, len(harnesses))
			labels := make([]string, 0, len(harnesses))
			for _, harness := range harnesses {
				target := filepath.Join(rootDir, filepath.FromSlash(harness.directory), "SKILL.md")
				if err := installAndurelSkill(filepath.Dir(target)); err != nil {
					return fmt.Errorf("install Andurel skill for %s: %w", harness.label, err)
				}
				installations = append(
					installations,
					skillInstallation{Harness: harness.name, Path: target},
				)
				labels = append(labels, harness.label)
			}

			report := skillReport{Name: "andurel", Installations: installations}
			if len(installations) == 1 {
				report.Path = installations[0].Path
			}
			return output.OK(cmd, report, "Installed Andurel skill for "+strings.Join(labels, ", "))
		},
	}
	installCmd.Flags().StringSliceVar(
		&requestedHarnesses,
		"harness",
		nil,
		"Harnesses to install for: codex, claude, pi, opencode, crush (comma-separated or repeated)",
	)
	cmd.AddCommand(installCmd)

	return cmd
}

func selectSkillHarnesses(cmd *cobra.Command, requested []string) ([]skillHarness, error) {
	if len(requested) > 0 {
		return parseSkillHarnessNames(requested)
	}

	opts, err := output.ParseOptions(cmd)
	if err != nil {
		return nil, err
	}
	if output.UsesStructuredOutput(opts) || opts.Quiet {
		return nil, skillHarnessError(
			"harness selection is required in non-interactive output modes",
			"Use --harness with one or more of: "+validSkillHarnessNames()+".",
		)
	}

	return promptSkillHarnesses(cmd)
}

func promptSkillHarnesses(cmd *cobra.Command) ([]skillHarness, error) {
	writer := cmd.OutOrStdout()
	if _, err := fmt.Fprintln(writer, "Select harnesses (comma-separated numbers):"); err != nil {
		return nil, err
	}
	for index, harness := range skillHarnesses {
		if _, err := fmt.Fprintf(writer, "  %d) %s\n", index+1, harness.label); err != nil {
			return nil, err
		}
	}
	if _, err := fmt.Fprint(writer, "Selection: "); err != nil {
		return nil, err
	}

	selection, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil {
		if err == io.EOF {
			return nil, skillHarnessError(
				"harness selection was cancelled",
				"Run the command again and select at least one harness.",
			)
		}
		return nil, fmt.Errorf("read harness selection: %w", err)
	}
	selection = strings.TrimSpace(selection)
	if selection == "" {
		return nil, skillHarnessError(
			"select at least one harness",
			"Enter one or more comma-separated numbers from the list.",
		)
	}

	names := make([]string, 0, len(skillHarnesses))
	for value := range strings.SplitSeq(selection, ",") {
		number, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || number < 1 || number > len(skillHarnesses) {
			return nil, skillHarnessError(
				fmt.Sprintf("invalid harness selection %q", strings.TrimSpace(value)),
				fmt.Sprintf("Enter comma-separated numbers from 1 to %d.", len(skillHarnesses)),
			)
		}
		names = append(names, skillHarnesses[number-1].name)
	}
	return parseSkillHarnessNames(names)
}

func parseSkillHarnessNames(names []string) ([]skillHarness, error) {
	selected := make([]skillHarness, 0, len(names))
	seen := make(map[string]bool, len(names))
	for _, value := range names {
		name := strings.ToLower(strings.TrimSpace(value))
		if name == "" {
			return nil, skillHarnessError(
				"harness name cannot be empty",
				"Choose one or more of: "+validSkillHarnessNames()+".",
			)
		}
		if seen[name] {
			continue
		}

		found := false
		for _, harness := range skillHarnesses {
			if harness.name == name {
				selected = append(selected, harness)
				seen[name] = true
				found = true
				break
			}
		}
		if !found {
			return nil, skillHarnessError(
				fmt.Sprintf("unknown harness %q", value),
				"Choose one or more of: "+validSkillHarnessNames()+".",
			)
		}
	}
	if len(selected) == 0 {
		return nil, skillHarnessError(
			"select at least one harness",
			"Choose one or more of: "+validSkillHarnessNames()+".",
		)
	}
	return selected, nil
}

func validSkillHarnessNames() string {
	names := make([]string, len(skillHarnesses))
	for index, harness := range skillHarnesses {
		names[index] = harness.name
	}
	return strings.Join(names, ", ")
}

func skillHarnessError(message, hint string) error {
	return output.NewError(output.CodeUsage, message, output.ExitUsage, hint)
}

// loadSkillReference resolves shorthand names and file paths to an embedded
// reference file and returns the canonical name and body.
func loadSkillReference(raw string) (string, string, error) {
	name := strings.TrimSpace(raw)
	name = strings.TrimPrefix(name, "./")
	name = strings.TrimPrefix(name, "andurel/")
	name = strings.TrimSuffix(name, ".md")
	name = strings.TrimPrefix(name, "references/")
	name = strings.TrimPrefix(name, "/")
	if name == "" || strings.Contains(name, "/") {
		return "", "", skillReferenceError(raw)
	}

	body, err := skills.ReadAndurelSkillFile("references/" + name + ".md")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", "", skillReferenceError(raw)
		}
		return "", "", fmt.Errorf("read skill reference %q: %w", raw, err)
	}

	return name, string(body), nil
}

func skillReferenceError(raw string) error {
	return output.NewError(
		output.CodeUsage,
		fmt.Sprintf("unknown skill reference %q", raw),
		output.ExitUsage,
		"Use one of: "+strings.Join(skillReferenceNames(), ", ")+
			"; run andurel skill show for SKILL.md.",
	)
}

func skillReferenceNames() []string {
	names := []string{}
	if err := skills.WalkAndurelSkillFiles(func(path string, _ []byte) error {
		rest, ok := strings.CutPrefix(path, "references/")
		if !ok {
			return nil
		}
		names = append(names, strings.TrimSuffix(rest, ".md"))
		return nil
	}); err != nil {
		return names
	}
	sort.Strings(names)
	return names
}

func installAndurelSkill(targetDir string) error {
	return skills.WalkAndurelSkillFiles(func(path string, data []byte) error {
		target := filepath.Join(targetDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
