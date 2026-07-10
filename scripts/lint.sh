#!/usr/bin/env bash
set -euo pipefail

readonly expected_version="2.12.2"
readonly repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

installed_version="$(golangci-lint version 2>&1)"
if [[ "${installed_version}" != *"has version ${expected_version} "* ]]; then
  echo "golangci-lint ${expected_version} is required" >&2
  echo "found: ${installed_version}" >&2
  exit 1
fi

lint_cache="$(mktemp -d "${TMPDIR:-/tmp}/andurel-golangci-lint.XXXXXX")"
go_cache="$(mktemp -d "${TMPDIR:-/tmp}/andurel-go-build.XXXXXX")"
cleanup() {
  rm -rf "${lint_cache}" "${go_cache}"
}
trap cleanup EXIT INT TERM

cd "${repo_root}"
GOLANGCI_LINT_CACHE="${lint_cache}" GOCACHE="${go_cache}" \
  golangci-lint run --config .golangci.yml "$@"
