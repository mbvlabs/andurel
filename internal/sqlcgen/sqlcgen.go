// Package sqlcgen implements an sqlc process plugin that, given the same
// CodeGenRequest sqlc-gen-go consumes, emits one Go file per andurel model
// with hook-wrapped CRUD methods on a zero-sized model struct.
//
// It is invoked by sqlc as a process plugin and reads a GenerateRequest
// from stdin / writes a GenerateResponse to stdout via the plugin SDK.
package sqlcgen

import (
	"context"
	"fmt"

	"github.com/sqlc-dev/plugin-sdk-go/codegen"
	"github.com/sqlc-dev/plugin-sdk-go/plugin"
)

// Run starts the plugin loop. It blocks until sqlc finishes communicating.
// On error the plugin SDK writes to stderr and exits with a non-zero status.
func Run() {
	codegen.Run(Generate)
}

// Generate is the single-shot handler. Exported so callers can drive it
// without the stdin/stdout protocol — useful for tests, smoke drivers, or
// alternate transports. The plugin's Run loop calls into it directly.
func Generate(_ context.Context, req *plugin.GenerateRequest) (*plugin.GenerateResponse, error) {
	cfg, err := loadConfig(req)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	groups, err := groupQueriesByModel(req.GetQueries())
	if err != nil {
		return nil, fmt.Errorf("group queries: %w", err)
	}

	files := make([]*plugin.File, 0, len(groups))
	for _, group := range groups {
		file, err := emitModelFile(cfg, group)
		if err != nil {
			return nil, fmt.Errorf("emit %s: %w", group.Model, err)
		}
		files = append(files, file)
	}

	return &plugin.GenerateResponse{Files: files}, nil
}
