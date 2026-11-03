package cli

import (
	"github.com/mbvlabs/andurel/v2/cli/internal/devserver"
	"github.com/mbvlabs/andurel/v2/layout/cmds"
	"github.com/mbvlabs/andurel/v2/layout/upgrade"
)

var runDevServerFunc = devserver.Run

var runFmtFunc = runFmt
var runGoFmtFunc = runGoFmt
var runGolinesFunc = runGolines
var syncSingleToolFunc = syncSingleTool
var ensureToolFunc = ensureToolInstalled
var downloadFromLockToolFunc = downloadFromLockTool

var installToolVersionAndLockFunc = installToolVersionAndLock
var resolveToolChecksumsFunc = cmds.ResolveURLTemplateChecksums

type cliUpgrader interface {
	Execute() (*upgrade.UpgradeReport, error)
}

var newUpgraderFunc = func(projectRoot string, opts upgrade.UpgradeOptions) (cliUpgrader, error) {
	return upgrade.NewUpgrader(projectRoot, opts)
}
