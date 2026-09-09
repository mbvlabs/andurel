package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mbvlabs/andurel/cli/output"
	"github.com/mbvlabs/andurel/layout/versions"
	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile"
)

const (
	packageStatusCurrent  = "current"
	packageStatusOutdated = "outdated"
	packageStatusUpdated  = "updated"
	packageStatusReplaced = "replaced"
)

type packageVersion struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Current string `json:"current"`
	Latest  string `json:"latest,omitempty"`
	Status  string `json:"status"`
	Replace string `json:"replace,omitempty"`
}

type packageUpdateReport struct {
	Action      string           `json:"action"`
	DryRun      bool             `json:"dry_run,omitempty"`
	Packages    []packageVersion `json:"packages"`
	CommandsRun []string         `json:"commands_run,omitempty"`
}

var applyAndurelPackageUpdatesFunc = applyAndurelPackageUpdates

func newPackagesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "packages",
		Aliases: []string{"package", "pkg"},
		Short:   "List or update Andurel packages in go.mod",
		Long: `List and update standalone Andurel packages required by this project.

This command updates github.com/mbvlabs/andurel/pkg/* module versions in
go.mod to the latest published releases. It does not change framework-owned
files; use andurel upgrade for that. andurel upgrade pins required packages to
the versions verified with the installed CLI instead of chasing @latest.

Only packages already required in go.mod are considered. Packages with a
replace directive are reported and left unchanged.`,
		Example: `  andurel packages
  andurel packages list --json
  andurel packages update --dry-run
  andurel packages update
  andurel packages update storage email`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPackagesList(cmd)
		},
	}
	setAgentMetadata(
		cmd,
		"maintenance",
		"Read-only by default. Use packages update to bump github.com/mbvlabs/andurel/pkg/* versions already required in go.mod.",
	)

	cmd.AddCommand(newPackagesListCommand(), newPackagesUpdateCommand())
	return cmd
}

func newPackagesListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List Andurel package versions in go.mod",
		Long:    "List github.com/mbvlabs/andurel/pkg/* modules required by this project and the latest published versions.",
		Example: `  andurel packages list
  andurel packages list --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPackagesList(cmd)
		},
	}
	setAgentMetadata(
		cmd,
		"maintenance",
		"Read-only comparison of required Andurel packages against proxy.golang.org @latest.",
	)
	return cmd
}

func newPackagesUpdateCommand() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "update [packages...]",
		Aliases: []string{"up"},
		Short:   "Update Andurel packages in go.mod to latest",
		Long: `Update github.com/mbvlabs/andurel/pkg/* modules already required in go.mod
to the latest stable versions published on proxy.golang.org.

Pass package names (storage, email, inertia, ...) or full module paths to
limit the update. With no arguments, every required Andurel package is
considered.

This runs go get and go mod tidy. Packages with a replace directive are
skipped. This does not upgrade framework-owned files; use andurel upgrade
for that. andurel upgrade pins required packages to the versions verified
with the installed CLI instead of chasing @latest.`,
		Example: `  andurel packages update --dry-run
  andurel packages update
  andurel packages update storage`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPackagesUpdate(cmd, dryRun, args)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be updated without changing go.mod")
	setAgentMetadata(
		cmd,
		"maintenance",
		"Mutates go.mod and go.sum. Use --dry-run --json before applying. Does not change framework-owned files.",
	)
	return cmd
}

func runPackagesList(cmd *cobra.Command) error {
	report, err := inspectAndurelPackages(cmd.Context(), nil)
	if err != nil {
		return err
	}
	report.Action = "list"
	return renderPackageReport(cmd, report, packageListSummary(report))
}

func runPackagesUpdate(cmd *cobra.Command, dryRun bool, names []string) error {
	report, err := inspectAndurelPackages(cmd.Context(), names)
	if err != nil {
		return err
	}
	report.Action = "update"
	report.DryRun = dryRun

	specs := outdatedPackageSpecs(report.Packages)
	if !dryRun && len(specs) > 0 {
		rootDir, err := findGoModRoot()
		if err != nil {
			return err
		}
		if err := applyAndurelPackageUpdatesFunc(cmd.Context(), rootDir, specs); err != nil {
			return err
		}
		for i, pkg := range report.Packages {
			if pkg.Status == packageStatusOutdated {
				report.Packages[i].Status = packageStatusUpdated
			}
		}
		report.CommandsRun = []string{"go get", "go mod tidy"}
	}

	return renderPackageReport(cmd, report, packageUpdateSummary(report, dryRun))
}

func inspectAndurelPackages(ctx context.Context, names []string) (packageUpdateReport, error) {
	rootDir, err := findGoModRoot()
	if err != nil {
		return packageUpdateReport{}, err
	}

	packages, err := listAndurelPackages(filepath.Join(rootDir, "go.mod"))
	if err != nil {
		return packageUpdateReport{}, err
	}
	if len(packages) == 0 {
		return packageUpdateReport{}, output.NewError(
			output.CodeConfigError,
			"no Andurel packages found in go.mod",
			output.ExitConfig,
			"This command updates github.com/mbvlabs/andurel/pkg/* modules already required by the project.",
		)
	}

	packages, err = filterAndurelPackages(packages, names)
	if err != nil {
		return packageUpdateReport{}, err
	}
	if err := fillLatestPackageVersions(ctx, packages); err != nil {
		return packageUpdateReport{}, err
	}

	return packageUpdateReport{Packages: packages}, nil
}

func listAndurelPackages(goModPath string) ([]packageVersion, error) {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, output.WrapError(
			output.CodeConfigError,
			fmt.Errorf("read go.mod: %w", err),
			output.ExitConfig,
			"Run this from a directory containing an Andurel project's go.mod file.",
		)
	}

	file, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return nil, output.WrapError(
			output.CodeConfigError,
			fmt.Errorf("parse go.mod: %w", err),
			output.ExitConfig,
			"Inspect go.mod and retry.",
		)
	}

	replaces := make(map[string]string)
	for _, replacement := range file.Replace {
		if replacement == nil || !strings.HasPrefix(replacement.Old.Path, versions.PkgPrefix) {
			continue
		}
		target := replacement.New.Path
		if replacement.New.Version != "" {
			target += " " + replacement.New.Version
		}
		replaces[replacement.Old.Path] = target
	}

	packages := make([]packageVersion, 0)
	for _, req := range file.Require {
		if req == nil || req.Indirect || !strings.HasPrefix(req.Mod.Path, versions.PkgPrefix) {
			continue
		}
		name := strings.TrimPrefix(req.Mod.Path, versions.PkgPrefix)
		if name == "" {
			continue
		}
		pkg := packageVersion{
			Name:    name,
			Path:    req.Mod.Path,
			Current: req.Mod.Version,
			Status:  packageStatusCurrent,
		}
		if target, ok := replaces[req.Mod.Path]; ok {
			pkg.Status = packageStatusReplaced
			pkg.Replace = target
		}
		packages = append(packages, pkg)
	}

	slices.SortFunc(packages, func(a, b packageVersion) int {
		return strings.Compare(a.Path, b.Path)
	})
	return packages, nil
}

func filterAndurelPackages(packages []packageVersion, names []string) ([]packageVersion, error) {
	if len(names) == 0 {
		return packages, nil
	}

	byName := make(map[string]packageVersion, len(packages))
	byPath := make(map[string]packageVersion, len(packages))
	for _, pkg := range packages {
		byName[pkg.Name] = pkg
		byPath[pkg.Path] = pkg
	}

	selected := make([]packageVersion, 0, len(names))
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		pkg, ok := byName[name]
		if !ok {
			pkg, ok = byPath[name]
		}
		if !ok {
			return nil, output.NewError(
				output.CodeUsage,
				fmt.Sprintf("Andurel package %q is not required in go.mod", name),
				output.ExitUsage,
				"Use andurel packages list to see packages required by this project.",
			)
		}
		if seen[pkg.Path] {
			continue
		}
		seen[pkg.Path] = true
		selected = append(selected, pkg)
	}
	return selected, nil
}

func fillLatestPackageVersions(ctx context.Context, packages []packageVersion) error {
	for i, pkg := range packages {
		if pkg.Status == packageStatusReplaced {
			continue
		}
		latest, err := lookupLatestModuleVersionFunc(ctx, pkg.Path)
		if err != nil {
			return output.WrapError(
				output.CodeExternalCommandFailed,
				fmt.Errorf("look up latest version for %s: %w", pkg.Path, err),
				output.ExitExternal,
				"Check network access to proxy.golang.org and retry.",
			)
		}
		packages[i].Latest = latest
		if newerAndurelVersion(pkg.Current, latest) {
			packages[i].Status = packageStatusOutdated
		} else {
			packages[i].Status = packageStatusCurrent
		}
	}
	return nil
}

func outdatedPackageSpecs(packages []packageVersion) []string {
	specs := make([]string, 0, len(packages))
	for _, pkg := range packages {
		if pkg.Status == packageStatusOutdated {
			specs = append(specs, pkg.Path+"@"+pkg.Latest)
		}
	}
	return specs
}

func applyAndurelPackageUpdates(ctx context.Context, dir string, specs []string) error {
	if len(specs) == 0 {
		return nil
	}

	getArgs := append([]string{"get"}, specs...)
	if err := runGoModCommand(ctx, dir, getArgs); err != nil {
		return err
	}
	return runGoModCommand(ctx, dir, []string{"mod", "tidy"})
}

func runGoModCommand(ctx context.Context, dir string, args []string) error {
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stdout = &stderr
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return output.WrapError(
			output.CodeExternalCommandFailed,
			fmt.Errorf("go %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String())),
			output.ExitExternal,
			"Inspect go.mod and retry the listed Go module command.",
		)
	}
	return nil
}

func renderPackageReport(cmd *cobra.Command, report packageUpdateReport, summary string) error {
	opts, err := output.ParseOptions(cmd)
	if err != nil {
		return err
	}
	if output.UsesStructuredOutput(opts) {
		return output.OK(cmd, report, summary)
	}
	if opts.Quiet {
		return nil
	}

	if _, err := fmt.Fprintln(cmd.OutOrStdout(), summary); err != nil {
		return err
	}
	if len(report.Packages) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(cmd.OutOrStdout()); err != nil {
		return err
	}
	width := packageNameWidth(report.Packages)
	for _, pkg := range report.Packages {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), formatPackageLine(pkg, width)); err != nil {
			return err
		}
	}
	return nil
}

func packageNameWidth(packages []packageVersion) int {
	width := 0
	for _, pkg := range packages {
		if len(pkg.Name) > width {
			width = len(pkg.Name)
		}
	}
	return width
}

func formatPackageLine(pkg packageVersion, width int) string {
	name := fmt.Sprintf("%-*s", width, pkg.Name)
	switch pkg.Status {
	case packageStatusReplaced:
		return fmt.Sprintf("  %s  %s  replaced (%s)", name, pkg.Current, pkg.Replace)
	case packageStatusOutdated, packageStatusUpdated:
		return fmt.Sprintf("  %s  %s -> %s  %s", name, pkg.Current, pkg.Latest, pkg.Status)
	default:
		return fmt.Sprintf("  %s  %s  current", name, pkg.Current)
	}
}

func packageListSummary(report packageUpdateReport) string {
	outdated := 0
	for _, pkg := range report.Packages {
		if pkg.Status == packageStatusOutdated {
			outdated++
		}
	}
	if outdated == 0 {
		return "All Andurel packages are at the latest versions"
	}
	if outdated == 1 {
		return "1 Andurel package has an update available"
	}
	return fmt.Sprintf("%d Andurel packages have updates available", outdated)
}

func packageUpdateSummary(report packageUpdateReport, dryRun bool) string {
	count := 0
	for _, pkg := range report.Packages {
		if pkg.Status == packageStatusOutdated || pkg.Status == packageStatusUpdated {
			count++
		}
	}
	switch {
	case count == 0:
		return "No Andurel package updates available"
	case dryRun && count == 1:
		return "Would update 1 Andurel package"
	case dryRun:
		return fmt.Sprintf("Would update %d Andurel packages", count)
	case count == 1:
		return "Updated 1 Andurel package"
	default:
		return fmt.Sprintf("Updated %d Andurel packages", count)
	}
}
