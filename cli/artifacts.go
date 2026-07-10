package cli

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mbvlabs/andurel/cli/output"
	"github.com/spf13/cobra"
)

type mutationReport struct {
	Action       string   `json:"action"`
	Resource     string   `json:"resource,omitempty"`
	DryRun       bool     `json:"dry_run,omitempty"`
	FilesCreated []string `json:"files_created"`
	FilesUpdated []string `json:"files_updated"`
	FilesDeleted []string `json:"files_deleted,omitempty"`
	RoutesAdded  []string `json:"routes_added"`
	CommandsRun  []string `json:"commands_run"`
	Warnings     []string `json:"warnings,omitempty"`
	Diff         string   `json:"diff,omitempty"`
}

type mutationOptions struct {
	Action      string
	Resource    string
	RootDir     string
	DryRun      bool
	Diff        bool
	CommandsRun []string
	Warnings    []string
	Breadcrumbs []output.Breadcrumb
	Run         func(rootDir string) error
}

type fileSnapshot map[string]fileState

type fileState struct {
	Hash    [32]byte
	Content []byte
	Mode    os.FileMode
}

func runMutation(cmd *cobra.Command, opts mutationOptions) error {
	if opts.Run == nil {
		return output.NewError(output.CodeUsage, "mutation runner is not configured", output.ExitUsage, "")
	}
	if opts.RootDir == "" {
		return output.NewError(output.CodeProjectNotFound, "project root is required", output.ExitProject, "")
	}

	outOpts, err := output.ParseOptions(cmd)
	if err != nil {
		return err
	}

	if opts.DryRun {
		return runDryMutation(cmd, outOpts, opts)
	}

	before, err := snapshotFilesForReport(opts.RootDir)
	if err != nil {
		return err
	}

	oldWD, _ := os.Getwd()
	if err := os.Chdir(opts.RootDir); err != nil {
		return err
	}
	runErr := runWithOptionalStdoutSilence(output.SuppressesHumanOutput(outOpts), func() error {
		return opts.Run(opts.RootDir)
	})
	_ = os.Chdir(oldWD)
	if runErr != nil {
		return runErr
	}

	after, err := snapshotFilesForReport(opts.RootDir)
	if err != nil {
		return err
	}
	report := buildMutationReport(opts, before, after)
	if output.SuppressesHumanOutput(outOpts) {
		return output.OK(cmd, report, mutationSummary(report), opts.Breadcrumbs...)
	}
	return nil
}

func runDryMutation(cmd *cobra.Command, outOpts output.Options, opts mutationOptions) (err error) {
	tempParent, err := os.MkdirTemp("", "andurel-dry-run-*")
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(tempParent))
	}()

	tempRoot := filepath.Join(tempParent, filepath.Base(opts.RootDir))
	if err := copyDir(opts.RootDir, tempRoot); err != nil {
		return err
	}

	before, err := snapshotFilesForReport(tempRoot)
	if err != nil {
		return err
	}

	oldWD, _ := os.Getwd()
	if err := os.Chdir(tempRoot); err != nil {
		return err
	}
	originalFindGoModRoot := findGoModRoot
	findGoModRoot = func() (string, error) {
		return tempRoot, nil
	}
	runErr := runWithOptionalStdoutSilence(output.SuppressesHumanOutput(outOpts), func() error {
		return opts.Run(tempRoot)
	})
	findGoModRoot = originalFindGoModRoot
	_ = os.Chdir(oldWD)
	if runErr != nil {
		return runErr
	}

	after, err := snapshotFilesForReport(tempRoot)
	if err != nil {
		return err
	}

	report := buildMutationReport(opts, before, after)
	report.DryRun = true
	report.Warnings = append(report.Warnings, "dry run only; no files were changed")
	if outOpts.Mode == output.ModeHuman && !outOpts.Quiet {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Dry run: %s\n", mutationSummary(report)); err != nil {
			return err
		}
		for _, path := range report.FilesCreated {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  create %s\n", path); err != nil {
				return err
			}
		}
		for _, path := range report.FilesUpdated {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  update %s\n", path); err != nil {
				return err
			}
		}
		for _, path := range report.FilesDeleted {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  delete %s\n", path); err != nil {
				return err
			}
		}
		return nil
	}
	return output.OK(cmd, report, mutationSummary(report), opts.Breadcrumbs...)
}

func buildMutationReport(opts mutationOptions, before, after fileSnapshot) mutationReport {
	report := mutationReport{
		Action:       opts.Action,
		Resource:     opts.Resource,
		DryRun:       opts.DryRun,
		FilesCreated: []string{},
		FilesUpdated: []string{},
		FilesDeleted: []string{},
		RoutesAdded:  []string{},
		CommandsRun:  append([]string(nil), opts.CommandsRun...),
		Warnings:     append([]string(nil), opts.Warnings...),
	}

	for path, afterState := range after {
		beforeState, exists := before[path]
		if !exists {
			report.FilesCreated = append(report.FilesCreated, path)
			if isRoutePath(path) {
				report.RoutesAdded = append(report.RoutesAdded, path)
			}
			continue
		}
		if beforeState.Hash != afterState.Hash {
			report.FilesUpdated = append(report.FilesUpdated, path)
			if isRoutePath(path) {
				report.RoutesAdded = append(report.RoutesAdded, path)
			}
		}
	}
	for path := range before {
		if _, exists := after[path]; !exists {
			report.FilesDeleted = append(report.FilesDeleted, path)
		}
	}

	sort.Strings(report.FilesCreated)
	sort.Strings(report.FilesUpdated)
	sort.Strings(report.FilesDeleted)
	sort.Strings(report.RoutesAdded)
	if opts.Diff {
		report.Diff = buildTextDiff(before, after, append(append([]string{}, report.FilesCreated...), report.FilesUpdated...))
	}

	return report
}

func mutationSummary(report mutationReport) string {
	verb := "Changed"
	if report.DryRun {
		verb = "Would change"
	}
	total := len(report.FilesCreated) + len(report.FilesUpdated) + len(report.FilesDeleted)
	if total == 0 {
		return fmt.Sprintf("%s completed with no file changes", report.Action)
	}
	return fmt.Sprintf("%s %d files for %s", verb, total, report.Action)
}

func isRoutePath(path string) bool {
	return strings.HasPrefix(filepath.ToSlash(path), "router/routes/")
}

func snapshotFilesForReport(root string) (fileSnapshot, error) {
	snapshot := fileSnapshot{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if shouldSkipReportPath(rel, entry) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		snapshot[rel] = fileState{
			Hash:    sha256.Sum256(content),
			Content: content,
			Mode:    info.Mode(),
		}
		return nil
	})
	return snapshot, err
}

func shouldSkipReportPath(rel string, entry os.DirEntry) bool {
	name := entry.Name()
	if name == ".git" || name == "bin" || name == "node_modules" || name == ".andurel-cache" {
		return true
	}
	return strings.HasPrefix(rel, ".git/")
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		rel = filepath.ToSlash(rel)
		if shouldSkipReportPath(rel, entry) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, filepath.FromSlash(rel))
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) (err error) {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := in.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := out.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	_, err = io.Copy(out, in)
	return err
}

func buildTextDiff(before, after fileSnapshot, paths []string) string {
	var b strings.Builder
	sort.Strings(paths)
	for _, path := range paths {
		afterState := after[path]
		if !isTextContent(afterState.Content) {
			continue
		}
		beforeContent := before[path].Content
		if len(beforeContent) > 0 && !isTextContent(beforeContent) {
			continue
		}
		b.WriteString("diff --git a/")
		b.WriteString(path)
		b.WriteString(" b/")
		b.WriteString(path)
		b.WriteByte('\n')
		if len(beforeContent) == 0 {
			b.WriteString("--- /dev/null\n")
		} else {
			b.WriteString("--- a/")
			b.WriteString(path)
			b.WriteByte('\n')
		}
		b.WriteString("+++ b/")
		b.WriteString(path)
		b.WriteByte('\n')
		b.WriteString(simpleLineDiff(string(beforeContent), string(afterState.Content)))
	}
	return b.String()
}

func runWithOptionalStdoutSilence(silence bool, run func() error) error {
	if !silence {
		return run()
	}

	originalStdout := os.Stdout
	originalStderr := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		return err
	}
	os.Stdout = writer
	os.Stderr = writer
	done := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(io.Discard, reader)
		done <- copyErr
	}()

	runErr := run()
	writerCloseErr := writer.Close()
	os.Stdout = originalStdout
	os.Stderr = originalStderr
	copyErr := <-done
	readerCloseErr := reader.Close()
	return errors.Join(runErr, writerCloseErr, copyErr, readerCloseErr)
}

func isTextContent(content []byte) bool {
	return !bytes.Contains(content, []byte{0})
}

func simpleLineDiff(before, after string) string {
	var b strings.Builder
	if before != "" {
		for line := range strings.SplitSeq(strings.TrimSuffix(before, "\n"), "\n") {
			b.WriteString("-")
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	if after != "" {
		for line := range strings.SplitSeq(strings.TrimSuffix(after, "\n"), "\n") {
			b.WriteString("+")
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return b.String()
}
