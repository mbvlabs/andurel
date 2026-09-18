#!/usr/bin/env bash
# Install pinned formatter/codegen tools. Versions must match layout/versions.
set -euo pipefail

readonly TEMPL_VERSION="${ANDUREL_TEMPL_VERSION:-v0.3.1020}"
readonly GOLINES_VERSION="${ANDUREL_GOLINES_VERSION:-v0.13.0}"
readonly GOIMPORTS_VERSION="${ANDUREL_GOIMPORTS_VERSION:-v0.44.0}"

go install "github.com/a-h/templ/cmd/templ@${TEMPL_VERSION}"
go install "github.com/segmentio/golines@${GOLINES_VERSION}"
go install "golang.org/x/tools/cmd/goimports@${GOIMPORTS_VERSION}"
