package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/v2/layout/versions"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
)

// narsilcRuntimeState describes how a project's narsilc runtime dependency
// relates to the configured narsilc generator version.
type narsilcRuntimeState struct {
	Status          string
	ModulePath      string
	ToolVersion     string
	RequiredVersion string
	Replacement     string
	Detail          string
}

const (
	narsilcRuntimeAligned      = "aligned"
	narsilcRuntimeMismatch     = "mismatch"
	narsilcRuntimeMissing      = "missing"
	narsilcRuntimeReplaced     = "replaced"
	narsilcRuntimeUnverifiable = "unverifiable"
)

// narsilcRuntimeReport is the structured form of a runtime check. It is
// intentionally independent of the human wording so both surfaces can share
// one source of truth.
type narsilcRuntimeReport struct {
	Status         string   `json:"status"`
	Module         string   `json:"module"`
	ToolVersion    string   `json:"tool_version,omitempty"`
	RuntimeVersion string   `json:"runtime_version,omitempty"`
	Replacement    string   `json:"replacement,omitempty"`
	Detail         string   `json:"detail,omitempty"`
	Remediation    []string `json:"remediation,omitempty"`
}

func (s narsilcRuntimeState) report() narsilcRuntimeReport {
	return narsilcRuntimeReport{
		Status:         s.Status,
		Module:         s.ModulePath,
		ToolVersion:    s.ToolVersion,
		RuntimeVersion: s.RequiredVersion,
		Replacement:    s.Replacement,
		Detail:         s.Detail,
		Remediation:    s.remediation(),
	}
}

func (s narsilcRuntimeState) remediation() []string {
	if s.Status != narsilcRuntimeMismatch || s.ModulePath == "" || s.ToolVersion == "" {
		return nil
	}
	return []string{
		fmt.Sprintf("go get %s@%s", s.ModulePath, s.ToolVersion),
		"go mod tidy",
	}
}

// warning returns the human guidance for a state that needs attention. A
// missing runtime requirement produces no warning: there is no pin to align
// and set-version must not inject one.
func (s narsilcRuntimeState) warning() string {
	switch s.Status {
	case narsilcRuntimeMismatch:
		return fmt.Sprintf(
			"narsilc runtime mismatch: %s requires %s but narsilc is %s",
			versions.NarsilcModulePath,
			s.RequiredVersion,
			s.ToolVersion,
		)
	case narsilcRuntimeReplaced:
		return fmt.Sprintf(
			"narsilc runtime is replaced in go.mod (%s); alignment with narsilc %s could not be verified",
			s.Replacement,
			s.ToolVersion,
		)
	case narsilcRuntimeUnverifiable:
		return fmt.Sprintf("could not verify narsilc runtime alignment: %s", s.Detail)
	default:
		return ""
	}
}

// inspectNarsilcRuntime reads go.mod with the Go module parser and compares
// the required narsilc module version to the generator version. It never
// mutates the module files. Replacements and unreadable or invalid module
// data are reported as unverifiable rather than as alignment.
func inspectNarsilcRuntime(projectRoot, toolVersion string) narsilcRuntimeState {
	state := narsilcRuntimeState{
		Status:      narsilcRuntimeUnverifiable,
		ModulePath:  versions.NarsilcModulePath,
		ToolVersion: strings.TrimSpace(toolVersion),
	}
	if canonical, ok := canonicalNarsilcVersion(toolVersion); ok {
		state.ToolVersion = canonical
	}

	goModPath := filepath.Join(projectRoot, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		state.Detail = fmt.Sprintf("could not read go.mod: %v", err)
		return state
	}
	file, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		state.Detail = fmt.Sprintf("could not parse go.mod: %v", err)
		return state
	}

	var requirement *modfile.Require
	for _, candidate := range file.Require {
		if candidate == nil || candidate.Mod.Path != versions.NarsilcModulePath {
			continue
		}
		requirement = candidate
		break
	}
	if requirement == nil {
		state.Status = narsilcRuntimeMissing
		state.Detail = fmt.Sprintf("go.mod does not require %s", versions.NarsilcModulePath)
		return state
	}
	state.RequiredVersion = requirement.Mod.Version

	if replacement := narsilcReplacementFor(file, requirement.Mod.Version); replacement != nil {
		state.Status = narsilcRuntimeReplaced
		state.Replacement = formatNarsilcReplacement(replacement)
		state.Detail = fmt.Sprintf(
			"go.mod replaces %s; alignment with the narsilc tool could not be verified",
			state.Replacement,
		)
		return state
	}

	required, requiredOK := canonicalNarsilcVersion(requirement.Mod.Version)
	tool, toolOK := canonicalNarsilcVersion(state.ToolVersion)
	if !requiredOK {
		state.Detail = fmt.Sprintf(
			"go.mod requires %s at invalid version %q",
			versions.NarsilcModulePath,
			requirement.Mod.Version,
		)
		return state
	}
	if !toolOK {
		state.Detail = fmt.Sprintf("narsilc tool version %q is not valid semantic versioning", state.ToolVersion)
		return state
	}
	state.RequiredVersion = required
	state.ToolVersion = tool
	if required == tool {
		state.Status = narsilcRuntimeAligned
		state.Detail = fmt.Sprintf("go.mod matches narsilc %s", tool)
		return state
	}

	state.Status = narsilcRuntimeMismatch
	state.Detail = fmt.Sprintf(
		"go.mod requires %s %s but narsilc is %s",
		versions.NarsilcModulePath,
		required,
		tool,
	)
	return state
}

func canonicalNarsilcVersion(version string) (string, bool) {
	version = strings.TrimSpace(version)
	if version == "" {
		return "", false
	}
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	canonical := semver.Canonical(version)
	return canonical, canonical != ""
}

// narsilcReplacementFor returns the replacement that applies to the required
// narsilc version. A replacement with an empty old version applies to every
// version of the module.
func narsilcReplacementFor(file *modfile.File, requiredVersion string) *modfile.Replace {
	for _, replacement := range file.Replace {
		if replacement == nil || replacement.Old.Path != versions.NarsilcModulePath {
			continue
		}
		if replacement.Old.Version == "" {
			return replacement
		}
		if narsilcVersionsEqual(replacement.Old.Version, requiredVersion) {
			return replacement
		}
	}
	return nil
}

func narsilcVersionsEqual(left, right string) bool {
	leftCanonical, leftOK := canonicalNarsilcVersion(left)
	rightCanonical, rightOK := canonicalNarsilcVersion(right)
	return leftOK && rightOK && leftCanonical == rightCanonical
}

func formatNarsilcReplacement(replacement *modfile.Replace) string {
	old := replacement.Old.Path
	if replacement.Old.Version != "" {
		old += " " + replacement.Old.Version
	}
	target := replacement.New.Path
	if replacement.New.Version != "" {
		target += " " + replacement.New.Version
	}
	if target == "" {
		target = "(local)"
	}
	return old + " => " + target
}
