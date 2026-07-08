package cli

import (
	"github.com/mbvlabs/andurel/generator"
	"github.com/mbvlabs/andurel/layout/upgrade"
)

type cliGenerator interface {
	GenerateModel(resourceName string, tableNameOverride string, skipFactory bool) error
	GenerateModelWithPK(resourceName string, tableNameOverride string, skipFactory bool, primaryKeyColumn string) error
	GenerateControllerWithActions(resourceName, namespace, tableName string, actions []string, inertia string, isAPI bool) error
	GenerateControllerWithActionsForModel(resourceName, namespace, modelName, tableName string, actions []string, inertia string, isAPI bool) error
	GenerateScaffold(resourceName, namespace, tableName string, skipFactory bool, primaryKeyColumn string, inertia string, isAPI bool) error
	SyncFactory(resourceName string, opts generator.FactorySyncOptions) (*generator.FactorySyncResult, error)
	SyncFactories(opts generator.FactorySyncOptions) ([]*generator.FactorySyncResult, error)
}

var newGenerator = func() (cliGenerator, error) {
	gen, err := generator.New()
	if err != nil {
		return nil, err
	}
	return &gen, nil
}

var runModelUpdateFunc = runModelUpdate
var runTemplFunc = runTempl
var runFmtFunc = runFmt
var runGoFmtFunc = runGoFmt
var runGolinesFunc = runGolines
var runTemplFmtFunc = runTemplFmt
var generateControllerWithActionsFunc = generateControllerWithActions
var syncSingleToolFunc = syncSingleTool
var downloadFromLockToolFunc = downloadFromLockTool

type cliUpgrader interface {
	Execute() (*upgrade.UpgradeReport, error)
}

var newUpgraderFunc = func(projectRoot string, opts upgrade.UpgradeOptions) (cliUpgrader, error) {
	return upgrade.NewUpgrader(projectRoot, opts)
}
