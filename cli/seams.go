package cli

import "github.com/mbvlabs/andurel/generator"

type cliGenerator interface {
	GenerateModel(resourceName string, tableNameOverride string, skipFactory bool) error
	GenerateModelWithPK(resourceName string, tableNameOverride string, skipFactory bool, primaryKeyColumn string) error
	GenerateControllerWithActions(resourceName, tableName string, withViews bool, actions []string, inertia string) error
	GenerateControllerWithActionsForModel(resourceName, modelName, tableName string, withViews bool, actions []string, inertia string) error
	GenerateScaffold(resourceName, tableName string, skipFactory bool, primaryKeyColumn string, inertia string) error
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
