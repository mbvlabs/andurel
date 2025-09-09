// Package layout provides functionality to scaffold a Go project with a predefined directory structure and files.
package layout

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const moduleName = "mbvlabs/andurel/layout/elements"

// Element represents a directory and its subdirectories.
type Element struct {
	RootDir string
	SubDirs []Element
}

// Layout defines the directory structure and files to be created.
var Layout = []Element{
	{
		RootDir: "assets",
		SubDirs: []Element{
			{
				RootDir: "css",
			},
			{
				RootDir: "js",
			},
		},
	},
	{
		RootDir: "cmd",
		SubDirs: []Element{
			{
				RootDir: "app",
			},
			{
				RootDir: "migrate",
			},
		},
	},
	{
		RootDir: "config",
	},
	{
		RootDir: "css",
	},
	{
		RootDir: "controllers",
	},
	{
		RootDir: "database",
		SubDirs: []Element{
			{
				RootDir: "migrations",
			},
			{
				RootDir: "queue",
			},
		},
	},
	{
		RootDir: "models",
	},
	{
		RootDir: "router",
		SubDirs: []Element{
			{
				RootDir: "cookies",
			},
			{
				RootDir: "middleware",
			},
			{
				RootDir: "routes",
			},
		},
	},
	{
		RootDir: "views",
		SubDirs: []Element{
			{
				RootDir: "internal",
				SubDirs: []Element{
					{RootDir: "layouts"},
				},
			},
		},
	},
}

// Scaffold creates the project structure in the specified target directory
func Scaffold(targetDir, projectName string) error {
	elementsDir := filepath.Join(filepath.Dir(getCurrentFile()), "elements")

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	if err := copyRootLevelFiles(elementsDir, targetDir, projectName); err != nil {
		return fmt.Errorf("failed to copy root-level files: %w", err)
	}

	if err := createGoMod(targetDir, projectName); err != nil {
		return fmt.Errorf("failed to create go.mod: %w", err)
	}

	for _, element := range Layout {
		if err := createElementStructure(targetDir, elementsDir, element, projectName); err != nil {
			return fmt.Errorf("failed to create element %s: %w", element.RootDir, err)
		}
	}

	if err := runGoModTidy(targetDir); err != nil {
		return fmt.Errorf("failed to run go mod tidy: %w", err)
	}

	if err := initializeGit(targetDir); err != nil {
		return fmt.Errorf("failed to initialize git: %w", err)
	}

	return nil
}

func createElementStructure(
	targetDir, elementsDir string,
	element Element,
	projectName string,
) error {
	elementTargetPath := filepath.Join(targetDir, element.RootDir)
	elementSourcePath := filepath.Join(elementsDir, element.RootDir)

	if err := os.MkdirAll(elementTargetPath, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", elementTargetPath, err)
	}

	if _, err := os.Stat(elementSourcePath); err == nil {
		if err := copyDirectoryContents(elementSourcePath, elementTargetPath, projectName); err != nil {
			return fmt.Errorf("failed to copy contents from %s: %w", elementSourcePath, err)
		}
	}

	for _, subElement := range element.SubDirs {
		if err := createElementStructure(elementTargetPath, elementSourcePath, subElement, projectName); err != nil {
			return fmt.Errorf("failed to create sub-element %s: %w", subElement.RootDir, err)
		}
	}

	return nil
}

func copyDirectoryContents(srcDir, destDir, projectName string) error {
	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(destDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0o755)
		}

		return copyFile(path, destPath, projectName)
	})
}

func copyFile(src, dest, projectName string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	content, err := io.ReadAll(srcFile)
	if err != nil {
		return err
	}

	contentStr := string(content)
	contentStr = strings.ReplaceAll(contentStr, moduleName, projectName)

	// Generate secure values for .env files
	if strings.HasSuffix(src, ".env.example") {
		contentStr = replaceSecureValues(contentStr)
	}

	_, err = destFile.WriteString(contentStr)
	return err
}

const goVersion = "1.24.4"

func createGoMod(targetDir, projectName string) error {
	goModPath := filepath.Join(targetDir, "go.mod")
	goModContent := fmt.Sprintf(
		"module %s\n\ngo %s\n\ntool (\n    github.com/a-h/templ/cmd/templ\n    github.com/sqlc-dev/sqlc/cmd/sqlc\n    github.com/pressly/goose/v3/cmd/goose\n    github.com/air-verse/air\n)\n",
		projectName,
		goVersion,
	)

	return os.WriteFile(goModPath, []byte(goModContent), 0o644)
}

func runGoModTidy(targetDir string) error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = targetDir
	return cmd.Run()
}

func copyRootLevelFiles(elementsDir, targetDir, projectName string) error {
	entries, err := os.ReadDir(elementsDir)
	if err != nil {
		return fmt.Errorf("failed to read elements directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			srcPath := filepath.Join(elementsDir, entry.Name())
			destPath := filepath.Join(targetDir, entry.Name())

			if err := copyFile(srcPath, destPath, projectName); err != nil {
				return fmt.Errorf("failed to copy file %s: %w", entry.Name(), err)
			}
		}
	}

	return nil
}

func initializeGit(targetDir string) error {
	cmd := exec.Command("git", "init")
	cmd.Dir = targetDir
	return cmd.Run()
}

func getCurrentFile() string {
	_, filename, _, _ := runtime.Caller(0)
	return filename
}

func replaceSecureValues(content string) string {
	// Generate secure random values
	sessionKey := generateRandomHex(64)
	sessionEncryptionKey := generateRandomHex(32)
	tokenSigningKey := generateRandomHex(32)
	passwordSalt := generateRandomHex(16)

	// Replace placeholder values with secure ones
	content = strings.ReplaceAll(content, "SESSION_KEY=session_key", "SESSION_KEY="+sessionKey)
	content = strings.ReplaceAll(
		content,
		"SESSION_ENCRYPTION_KEY=session_encryption_key",
		"SESSION_ENCRYPTION_KEY="+sessionEncryptionKey,
	)
	content = strings.ReplaceAll(
		content,
		"TOKEN_SIGNING_KEY=token_signing_key",
		"TOKEN_SIGNING_KEY="+tokenSigningKey,
	)
	content = strings.ReplaceAll(
		content,
		"PASSWORD_SALT=password_salt",
		"PASSWORD_SALT="+passwordSalt,
	)

	return content
}

func generateRandomHex(bytes int) string {
	randomBytes := make([]byte, bytes)
	rand.Read(randomBytes)
	return hex.EncodeToString(randomBytes)
}
