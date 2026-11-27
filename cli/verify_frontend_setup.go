package cli

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

//go:embed verify_frontend_setup.mjs
var verifyFrontendSetupScript string

const frontendSetupRule = "frontend-setup"

type frontendSetupResult struct {
	Violations          []verifyViolation `json:"violations"`
	Blocked             string            `json:"blocked"`
	EntryPoints         []string          `json:"entryPoints"`
	MissingDependencies bool              `json:"missingDependencies"`
}

// frontendSetupApplicable deliberately does not infer success from absent files.
// An Inertia lock or dependency makes the contract applicable. An unrelated
// Vite frontend must not opt a Go project into the Inertia framework contract.
func frontendSetupApplicable(rootDir string) bool {
	if state, err := detectSetupState(rootDir); err == nil && state.Inertia {
		return true
	}
	data, err := os.ReadFile(filepath.Join(rootDir, "package.json"))
	if err != nil {
		return false
	}
	var manifest struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if json.Unmarshal(data, &manifest) != nil {
		return false
	}
	for _, deps := range []map[string]string{manifest.Dependencies, manifest.DevDependencies} {
		for name := range deps {
			if strings.HasPrefix(name, "@inertiajs/") {
				return true
			}
		}
	}
	return false
}

func verifyFrontendSetup(rootDir string) verifyStep {
	if !frontendSetupApplicable(rootDir) {
		return verifyStep{Name: "frontend", Status: verifyStatusSkipped,
			Reason: "no Inertia frontend configured"}
	}
	step := verifyStepOK("frontend")
	violations, routeBlocked := verifyFrontendAssetRoutes(rootDir)
	step.Violations = append(step.Violations, violations...)
	result := runFrontendSetupConfig(rootDir)
	step.Violations = append(step.Violations, result.Violations...)
	cssBlocked := ""
	entryBlocked := ""
	if len(result.EntryPoints) > 0 {
		cssViolations, reason := verifyFrontendCSS(rootDir, result.EntryPoints)
		step.Violations = append(step.Violations, cssViolations...)
		cssBlocked = reason
		entry, envKey, reason := frontendRendererEntry(rootDir)
		entryBlocked = reason
		if reason == "" && envKey != "" {
			env, err := frontendSetupEnvironment(rootDir)
			if err != nil {
				entryBlocked = "cannot inspect the renderer entry environment; make the project .env readable and valid"
			} else {
				for _, item := range env {
					if key, value, ok := strings.Cut(item, "="); ok && key == envKey {
						entry = value
						break
					}
				}
			}
		}
		if entryBlocked == "" {
			absoluteRoot, err := filepath.Abs(rootDir)
			if err != nil {
				entryBlocked = "cannot resolve the project directory for renderer entry inspection"
			} else {
				wanted := filepath.Clean(filepath.Join(absoluteRoot, filepath.FromSlash(entry)))
				found := false
				for _, input := range result.EntryPoints {
					if filepath.Clean(input) == wanted {
						found = true
						break
					}
				}
				if !found {
					step.Violations = append(step.Violations, frontendViolation("app", "renderer EntryPoint is not a Vite production input", "Use the same frontend entry in inertia.RendererConfig.EntryPoint and Vite build.rollupOptions.input."))
				}
			}
		}
	} else if result.Blocked == "" && len(result.Violations) == 0 {
		cssBlocked = "Vite did not report frontend entry points; CSS entry wiring could not be verified"
	}
	if len(step.Violations) > 0 {
		step.Status, step.OK = verifyStatusFail, false
	}
	for _, reason := range []string{routeBlocked, result.Blocked, cssBlocked, entryBlocked} {
		if reason == "" {
			continue
		}
		step.OK = false
		if step.Status != verifyStatusFail {
			step.Status = verifyStatusBlocked
		}
		appendVerifyDetail(&step, reason)
	}
	if step.Status == verifyStatusBlocked {
		step.Reason = "frontend setup could not be fully verified; resolve the checks listed in detail"
	}
	return step
}

func runFrontendSetupConfig(rootDir string) frontendSetupResult {
	node, err := exec.LookPath("node")
	if err != nil {
		return frontendSetupResult{Blocked: "Node.js is missing; install the project's supported Node.js version, then run " + frontendPackageManager(rootDir) + " install"}
	}
	rootDir, err = filepath.Abs(rootDir)
	if err != nil {
		return frontendSetupResult{Blocked: "cannot resolve the project directory: " + err.Error()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, "--input-type=module", "-", rootDir)
	cmd.Dir = rootDir
	cmd.Env, err = frontendSetupEnvironment(rootDir)
	if err != nil {
		return frontendSetupResult{Blocked: "cannot inspect the development environment; make the project .env readable and valid"}
	}
	cmd.Stdin = strings.NewReader(verifyFrontendSetupScript)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	// Configurations can log environment data. Only expose structured checker
	// diagnostics, never arbitrary project/plugin stdout or stderr.
	err = cmd.Run()
	if ctx.Err() != nil {
		return frontendSetupResult{Blocked: "Vite configuration inspection timed out; ensure the configuration resolves without starting services"}
	}
	const marker = "ANDUREL_FRONTEND_SETUP="
	for line := range strings.SplitSeq(stdout.String(), "\n") {
		if !strings.HasPrefix(line, marker) {
			continue
		}
		var result frontendSetupResult
		if json.Unmarshal([]byte(strings.TrimPrefix(line, marker)), &result) == nil {
			if result.MissingDependencies {
				result.Blocked += "\nRun: " + frontendPackageManager(rootDir) + " install"
			}
			return result
		}
	}
	return frontendSetupResult{Blocked: fmt.Sprintf("Vite configuration inspection did not return a report (%v); ensure Node.js and project Vite dependencies are installed with %s install", err, frontendPackageManager(rootDir))}
}

// Match andurel run's .env loading without mutating the CLI's global environment.
// Values are passed only to the trusted project config, never into diagnostics.
func frontendSetupEnvironment(rootDir string) ([]string, error) {
	values, err := godotenv.Read(filepath.Join(rootDir, ".env"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	env := os.Environ()
	for key, value := range values {
		if _, exists := os.LookupEnv(key); !exists {
			env = append(env, key+"="+value)
		}
	}
	return env, nil
}
