#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

while IFS= read -r module_file; do
  module_dir="$(dirname "$module_file")"
  echo "vetting ${module_dir#"$repo_root"/}"
  (
    cd "$module_dir"
    GOWORK=off go mod download
    GOWORK=off go vet ./...
  )
done < <(find "$repo_root/pkg" -mindepth 2 -maxdepth 2 -name go.mod -print | sort)
