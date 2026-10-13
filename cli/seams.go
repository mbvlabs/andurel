package cli

import (
	"github.com/mbvlabs/andurel/v2/layout/cmds"
	"github.com/mbvlabs/andurel/v2/layout/upgrade"
)

var runTemplFunc = runTempl
var runFmtFunc = runFmt
var runGoFmtFunc = runGoFmt
var runGolinesFunc = runGolines
var runTemplFmtFunc = runTemplFmt
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
