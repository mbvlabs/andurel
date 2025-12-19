# Andurel Project Upgrade Mechanism - Implementation Plan

## Executive Summary

Implement a Rails-style `andurel upgrade` command that brings existing projects up to date with the latest framework templates. The system will re-generate templates and intelligently merge changes while preserving user modifications.

**Primary Approach**: Template Re-Generation with Smart Merging (Rails app:update style)

## Why This Approach

Based on project requirements:
- **Holistic updates**: Users want to bring entire projects current, not cherry-pick changes
- **Weekly/monthly cadence**: Needs to be fast and simple to run frequently during pre-v1
- **Basic git users**: Must automate safety (backups, conflict handling) without requiring advanced git knowledge
- **Moderate documentation**: Show diffs of what changed, don't need detailed manifests per improvement

The re-generation approach leverages andurel's existing template system and provides a familiar workflow similar to Rails app:update.

## Core Design

### How It Works

1. **Backup First**: Automatically create git commit/branch before any changes
2. **Shadow Generation**: Re-run template generation in temporary directory using latest templates
3. **Smart Detection**: Use git to identify which files users have modified vs untouched generated files
4. **Intelligent Merging**:
   - **Unmodified files**: Replace directly with new template (safe, automatic)
   - **User-modified files**: Attempt 3-way merge (old template → user changes ← new template)
   - **Conflicts**: Mark for manual resolution with clear instructions
5. **Update Lock**: Record new template version in andurel.lock

### User Experience

```bash
# Simple command to upgrade
andurel upgrade

# Output shows progress and requires minimal interaction
Upgrading from v0.5.0 to v0.6.0...
✓ Created backup commit (abc123)
✓ Generated fresh templates
✓ Analyzing 127 files...

Changes to apply:
  • 84 files: Safe to update (unchanged)
  • 12 files: Auto-merged (user changes preserved)
  • 3 files: Need review (conflicts)

Apply changes? [Y/n] Y

✓ Updated 96 files
⚠ 3 files need manual review:
  - views/home.templ (conflict markers added)

Fix conflicts and run: andurel upgrade finalize
```

### Safety Guarantees

- Always creates backup before making changes
- Never overwrites files without user confirmation
- Users can abort and restore from backup
- All changes visible in git diff
- Dry-run mode to preview without changes

## Implementation Plan

### Phase 1: Foundation & Lock File Enhancement

**Files to modify:**
- `/home/mbv/work/andurel/layout/lock.go`

**Changes:**
```go
type AndurelLock struct {
    Version         string
    Extensions      map[string]*Extension
    Tools           map[string]*Tool
    TemplateVersion string  // NEW: Track template version that generated this project
    ScaffoldConfig  *ScaffoldConfig  // NEW: Store original scaffold parameters
}

type ScaffoldConfig struct {
    ProjectName   string
    Repository    string
    Database      string
    CSSFramework  string
    Extensions    []string
}
```

**Tasks:**
1. Add TemplateVersion field (use git commit hash or version tag)
2. Add ScaffoldConfig to store original generation parameters
3. Update Scaffold() function to populate these fields during project creation
4. Ensure new projects get this metadata in andurel.lock

### Phase 2: Git Integration Layer

**Files to create:**
- `/home/mbv/work/andurel/layout/upgrade/git.go`

**Functionality:**
```go
type GitAnalyzer struct {
    projectRoot string
}

// Create backup before upgrade
func (g *GitAnalyzer) CreateBackup() (string, error)

// Detect which generated files users have modified
func (g *GitAnalyzer) GetModifiedFiles() (map[string]bool, error)

// Check if working tree is clean
func (g *GitAnalyzer) IsClean() (bool, error)

// Restore from backup if upgrade fails
func (g *GitAnalyzer) RestoreBackup(backupRef string) error
```

**Implementation notes:**
- Use git commands via exec.Command
- For modified files: `git diff --name-only <first-commit>..HEAD`
- For backup: `git stash push -m "andurel-upgrade-backup"` or create commit
- Keep it simple - basic git operations only
- Provide clear error messages if git not initialized

### Phase 3: Shadow Template Generation

**Files to create:**
- `/home/mbv/work/andurel/layout/upgrade/generator.go`

**Functionality:**
```go
type TemplateGenerator struct {
    targetVersion string
}

// Generate fresh templates in temporary directory
func (g *TemplateGenerator) Generate(config ScaffoldConfig) (shadowDir string, err error)

// Clean up temporary directory
func (g *TemplateGenerator) Cleanup(shadowDir string) error
```

**Implementation:**
- Create temp directory: `.andurel-upgrade-<timestamp>/`
- Call existing Scaffold() function with stored config
- Use latest embedded templates from binary
- Return path to generated shadow project

### Phase 4: File Comparison & Merging

**Files to create:**
- `/home/mbv/work/andurel/layout/upgrade/differ.go`
- `/home/mbv/work/andurel/layout/upgrade/merger.go`

**Differ functionality:**
```go
type FileDiffer struct {}

type DiffResult struct {
    Path          string
    Status        DiffStatus  // Identical, Changed, UserModified, Conflict
    UnifiedDiff   string
}

func (d *FileDiffer) Compare(oldPath, newPath, userPath string) (*DiffResult, error)
```

**Merger functionality:**
```go
type FileMerger struct {}

type MergeResult struct {
    Success      bool
    Content      []byte
    HasConflicts bool
    ConflictInfo string
}

func (m *FileMerger) Merge(oldContent, userContent, newContent []byte) (*MergeResult, error)
```

**Merge strategy:**
1. If user file == old template: Use new template (safe replace)
2. If user file != old template: Attempt 3-way merge
3. If merge succeeds without conflicts: Apply merged content
4. If merge has conflicts: Add conflict markers like git, flag for manual review

**Note**: For MVP, can use simple line-based diff/merge. Don't need sophisticated algorithms initially.

### Phase 5: Upgrade Orchestrator

**Files to create:**
- `/home/mbv/work/andurel/layout/upgrade/orchestrator.go`

**Main upgrade flow:**
```go
type Upgrader struct {
    projectRoot string
    lock        *layout.AndurelLock
    git         *GitAnalyzer
    generator   *TemplateGenerator
    differ      *FileDiffer
    merger      *FileMerger
    opts        UpgradeOptions
}

type UpgradeOptions struct {
    DryRun       bool
    Auto         bool  // Accept all safe changes without prompting
    TargetVersion string
}

func (u *Upgrader) Execute() (*UpgradeReport, error)
```

**Execute() steps:**
1. Validate preconditions (git repo exists, lock file present)
2. Check current vs target template version
3. Create backup (git commit or stash)
4. Generate shadow templates
5. Walk both directories, for each file:
   - Skip user-created files (not in template mappings)
   - Compare with shadow version
   - If identical: skip
   - If unmodified by user: queue for replacement
   - If user-modified: queue for merge
6. Show summary to user, prompt for confirmation (unless --auto)
7. Apply changes:
   - Replace unmodified files
   - Merge user-modified files
   - Write conflict markers for failed merges
8. Update andurel.lock with new TemplateVersion
9. Clean up shadow directory
10. Run post-upgrade hooks (go mod tidy, templ generate, etc.)
11. Show final report

### Phase 6: CLI Commands

**Files to create:**
- `/home/mbv/work/andurel/cli/upgrade.go`

**Files to modify:**
- `/home/mbv/work/andurel/cli/cli.go` (add upgrade command)

**Commands:**
```go
// Main upgrade command
andurel upgrade
andurel upgrade --dry-run
andurel upgrade --auto

// Finalize after resolving conflicts
andurel upgrade finalize

// Abort and restore backup
andurel upgrade abort

// Show upgrade status
andurel upgrade status
```

**Implementation:**
- Use cobra for command structure
- Simple prompts with bufio.Reader (no fancy TUI needed for MVP)
- Clear progress output with status indicators
- Helpful error messages and recovery instructions

### Phase 7: Post-Upgrade Hooks

**Files to modify:**
- `/home/mbv/work/andurel/layout/upgrade/orchestrator.go`

**Hooks to run after successful upgrade:**
1. `go mod tidy` - Update dependencies
2. `andurel sync` - Ensure tools are current
3. `templ generate` - Regenerate templ code
4. `go fmt ./...` - Format code
5. Optionally: `go vet ./...` - Check for issues

**Error handling:**
- If hooks fail, don't roll back upgrade (already applied)
- Show clear error message with remediation steps
- User can fix and re-run hooks manually

## File Structure

```
/home/mbv/work/andurel/
├── cli/
│   ├── upgrade.go (NEW)
│   └── cli.go (MODIFY - add upgrade cmd)
├── layout/
│   ├── lock.go (MODIFY - add TemplateVersion, ScaffoldConfig)
│   ├── layout.go (MODIFY - populate new lock fields)
│   └── upgrade/
│       ├── orchestrator.go (NEW)
│       ├── git.go (NEW)
│       ├── generator.go (NEW)
│       ├── differ.go (NEW)
│       └── merger.go (NEW)
```

## Alternative Approach Considered

**Manifest-Driven Incremental Patches** was also evaluated but not chosen because:
- User wants holistic upgrades, not granular patch selection
- Requires manifest generation and maintenance overhead
- More complex for users to understand and use
- Better suited for power-users who want fine-grained control

However, elements could be incorporated later:
- Track upgrade history in lock file (which patches applied)
- Generate changelogs showing what changed between versions
- Add `--only=<path>` flag to upgrade specific files/directories

## Testing Strategy

### Unit Tests
- Git operations (mocked)
- File diffing and merging
- Lock file serialization/deserialization

### Integration Tests
1. Create test project with andurel v0.5.0
2. Modify some files (simulate user changes)
3. Run upgrade to v0.6.0
4. Verify:
   - Unmodified files updated
   - User changes preserved
   - Lock file updated
   - No unintended changes

### Manual Testing Scenarios
1. Clean project (no user changes) → should upgrade cleanly
2. Modified views → should preserve user templates
3. Modified controllers → should merge changes
4. Conflicting changes → should add conflict markers
5. Upgrade abort → should restore backup
6. Multiple version jumps → should work (v0.5.0 → v0.7.0)

## Migration Path

### For existing andurel projects without TemplateVersion
1. When upgrade command runs, detect missing TemplateVersion
2. Prompt user: "Cannot determine project version. What version was this project generated with?"
3. User provides version (or "unknown")
4. If unknown: Show diff of ALL files, treat as if upgrading from v0.1.0
5. Update lock file with provided/assumed version

### For andurel binary itself
1. Add version constant: `const CurrentTemplateVersion = "v0.6.0"` or use git hash
2. During Scaffold(), record this version in lock file
3. During upgrade, compare lock's TemplateVersion with CurrentTemplateVersion

## SafeURL Example Walkthrough

User has project from v0.5.0 (before SafeURL methods existed).

```bash
$ andurel upgrade

Upgrading from v0.5.0 to v0.6.0...
✓ Created backup commit (f9d3a1e)
✓ Generated fresh templates
✓ Analyzing 127 files...

router/routes/routes.go
  Status: Unmodified by user
  Changes: Add SafeURL() methods (6 new methods, 1 new import)
  → Will replace with new version

views/login.templ
  Status: Modified by user (you added custom styling)
  Changes: Base template unchanged
  → Your changes will be preserved

Apply these changes? [Y/n] Y

✓ Updated router/routes/routes.go
✓ Preserved views/login.templ
✓ 125 other files checked (0 changed)
✓ Updated andurel.lock

Upgrade complete!
Your project is now on template version v0.6.0.

New features available:
  • Route SafeURL() convenience methods
  • Use routes.SessionNew.SafeURL() in templates instead of templ.SafeURL(routes.SessionNew.URL())
```

The user's `router/routes/routes.go` now has the SafeURL methods, and they can start using them in their templates.

## Future Enhancements

### Post-MVP improvements to consider:
1. **Selective upgrades**: `andurel upgrade --only=router/`
2. **Change summaries**: Generate CHANGELOG.md showing what improved
3. **Interactive diff viewer**: Show syntax-highlighted diffs inline
4. **Rollback command**: `andurel upgrade rollback` to undo last upgrade
5. **Upgrade notifications**: `andurel status` shows "Upgrade available: v0.6.0 → v0.7.0"
6. **Extension upgrades**: Handle extensions that get updated separately
7. **Template conflicts**: Better strategies for files with complex user modifications

## Critical Files Reference

### Existing files to understand:
- `/home/mbv/work/andurel/layout/layout.go` - Main Scaffold() function
- `/home/mbv/work/andurel/layout/lock.go` - Lock file structure
- `/home/mbv/work/andurel/layout/template_data.go` - Template rendering data
- `/home/mbv/work/andurel/layout/templates/templates.go` - Embedded templates
- `/home/mbv/work/andurel/cli/new_project.go` - Project creation CLI

### New files to create:
- `/home/mbv/work/andurel/layout/upgrade/orchestrator.go` - Main upgrade logic
- `/home/mbv/work/andurel/layout/upgrade/git.go` - Git operations
- `/home/mbv/work/andurel/layout/upgrade/generator.go` - Shadow generation
- `/home/mbv/work/andurel/layout/upgrade/differ.go` - File comparison
- `/home/mbv/work/andurel/layout/upgrade/merger.go` - 3-way merge
- `/home/mbv/work/andurel/cli/upgrade.go` - CLI commands

## Success Criteria

The upgrade mechanism is successful when:
1. ✅ Users can upgrade projects with one command: `andurel upgrade`
2. ✅ Unmodified generated files are safely updated
3. ✅ User modifications are preserved during upgrade
4. ✅ Conflicts are clearly marked and easy to resolve
5. ✅ Users can safely abort if something goes wrong
6. ✅ Works for projects generated with older andurel versions
7. ✅ Clear feedback about what changed and why
8. ✅ No data loss or corruption during upgrade process
