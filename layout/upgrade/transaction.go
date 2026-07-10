package upgrade

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type failureInjector func(operation, path string) error

// transactionRuntime holds injectable transaction behavior behind a pointer so
// Upgrader retains its established comparability contract.
type transactionRuntime struct {
	failureInjector failureInjector
}

func validatePlannedFiles(plan *upgradePlan) error {
	for _, file := range plan.files {
		if file.remove || !strings.HasSuffix(file.path, ".go") {
			continue
		}
		if _, err := parser.ParseFile(token.NewFileSet(), file.path, file.after, parser.AllErrors); err != nil {
			return fmt.Errorf("validate %s: %w", file.path, err)
		}
	}
	return nil
}

func (u *Upgrader) applyPlan(plan *upgradePlan) (err error) {
	if err := validatePlannedFiles(plan); err != nil {
		return err
	}
	txnDir, err := os.MkdirTemp(u.projectRoot, ".andurel-upgrade-")
	if err != nil {
		return fmt.Errorf("create upgrade staging directory: %w", err)
	}
	writesStarted := false
	defer func() {
		if err == nil {
			return
		}
		if writesStarted {
			if rollbackErr := rollbackPlan(u.projectRoot, plan); rollbackErr != nil {
				err = fmt.Errorf("%w; rollback failed: %v", err, rollbackErr)
			}
		}
		_ = os.RemoveAll(txnDir)
	}()

	for _, file := range plan.files {
		if err := u.inject("stage", file.path); err != nil {
			return fmt.Errorf("stage %s: %w", file.path, err)
		}
		stagePath := filepath.Join(txnDir, "stage", filepath.FromSlash(file.path))
		if file.remove {
			continue
		}
		if err := writeDurableFile(stagePath, file.after, file.mode, u.inject); err != nil {
			return fmt.Errorf("stage %s: %w", file.path, err)
		}
	}
	if err := u.inject("validation", txnDir); err != nil {
		return fmt.Errorf("validate staged upgrade: %w", err)
	}
	if err := validatePlannedFiles(plan); err != nil {
		return err
	}

	for _, file := range plan.files {
		if err := u.inject("backup", file.path); err != nil {
			return fmt.Errorf("backup %s: %w", file.path, err)
		}
		if file.created {
			continue
		}
		backupPath := filepath.Join(txnDir, "backup", filepath.FromSlash(file.path))
		if err := writeDurableFile(backupPath, file.before, file.mode, u.inject); err != nil {
			return fmt.Errorf("backup %s: %w", file.path, err)
		}
	}

	writesStarted = true
	for _, file := range plan.files {
		if file.isLock {
			continue
		}
		if err := u.applyFile(file); err != nil {
			return err
		}
	}
	if err := validateAppliedFiles(u.projectRoot, plan, false); err != nil {
		return fmt.Errorf("validate applied project files: %w", err)
	}

	for _, file := range plan.files {
		if !file.isLock {
			continue
		}
		if err := u.applyFile(file); err != nil {
			return err
		}
	}
	if err := u.inject("post-write-validation", u.projectRoot); err != nil {
		return fmt.Errorf("post-write validation: %w", err)
	}
	if err := validateAppliedFiles(u.projectRoot, plan, true); err != nil {
		return fmt.Errorf("post-write validation: %w", err)
	}
	if err := u.inject("cleanup", txnDir); err != nil {
		return fmt.Errorf("cleanup upgrade backup: %w", err)
	}
	if err := os.RemoveAll(txnDir); err != nil {
		return fmt.Errorf("cleanup upgrade backup: %w", err)
	}
	return nil
}

func validateAppliedFiles(root string, plan *upgradePlan, includeLock bool) error {
	for _, file := range plan.files {
		if file.isLock && !includeLock {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(file.path))
		content, err := os.ReadFile(path)
		if file.remove {
			if !os.IsNotExist(err) {
				return fmt.Errorf("%s was not deleted", file.path)
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("read %s: %w", file.path, err)
		}
		if !slices.Equal(content, file.after) {
			return fmt.Errorf("%s does not match staged content", file.path)
		}
		if strings.HasSuffix(file.path, ".go") {
			if _, err := parser.ParseFile(token.NewFileSet(), file.path, content, parser.AllErrors); err != nil {
				return fmt.Errorf("parse %s: %w", file.path, err)
			}
		}
	}
	return nil
}

func (u *Upgrader) applyFile(file plannedFile) error {
	if err := u.inject("write", file.path); err != nil {
		return fmt.Errorf("write %s: %w", file.path, err)
	}
	fullPath := filepath.Join(u.projectRoot, filepath.FromSlash(file.path))
	if file.remove {
		if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("delete %s: %w", file.path, err)
		}
		return syncDirectory(filepath.Dir(fullPath), u.inject, file.path)
	}
	if err := atomicReplace(fullPath, file.after, file.mode, u.inject, file.path); err != nil {
		return fmt.Errorf("replace %s: %w", file.path, err)
	}
	return nil
}

func (u *Upgrader) inject(operation, path string) error {
	if u.transaction == nil || u.transaction.failureInjector == nil {
		return nil
	}
	return u.transaction.failureInjector(operation, path)
}

func writeDurableFile(path string, content []byte, mode os.FileMode, inject failureInjector) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			_ = file.Close()
		}
	}()
	if _, err := file.Write(content); err != nil {
		return err
	}
	if inject != nil {
		if err := inject("sync", path); err != nil {
			return err
		}
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if inject != nil {
		if err := inject("close", path); err != nil {
			return err
		}
	}
	if err := file.Close(); err != nil {
		return err
	}
	closed = true
	return nil
}

func atomicReplace(path string, content []byte, mode os.FileMode, inject failureInjector, reportPath string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".andurel-replacement-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	closed := false
	defer func() {
		if !closed {
			_ = temporary.Close()
		}
		_ = os.Remove(temporaryPath)
	}()
	if err := temporary.Chmod(mode); err != nil {
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		return err
	}
	if inject != nil {
		if err := inject("sync", reportPath); err != nil {
			return err
		}
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if inject != nil {
		if err := inject("close", reportPath); err != nil {
			return err
		}
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	closed = true
	if inject != nil {
		if err := inject("rename", reportPath); err != nil {
			return err
		}
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path), inject, reportPath)
}

func syncDirectory(path string, inject failureInjector, reportPath string) error {
	if inject != nil {
		if err := inject("directory-sync", reportPath); err != nil {
			return err
		}
	}
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()
		return err
	}
	return directory.Close()
}

func rollbackPlan(root string, plan *upgradePlan) error {
	var rollbackErrors []error
	files := slices.Clone(plan.files)
	slices.Reverse(files)
	for _, file := range files {
		path := filepath.Join(root, filepath.FromSlash(file.path))
		if file.created {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("remove created %s: %w", file.path, err))
			}
			continue
		}
		if err := atomicReplace(path, file.before, file.mode, nil, file.path); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("restore %s: %w", file.path, err))
		}
	}
	if len(rollbackErrors) > 0 {
		return fmt.Errorf("%v", rollbackErrors)
	}
	return nil
}
