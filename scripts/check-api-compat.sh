#!/usr/bin/env bash
set -euo pipefail

if (( $# != 2 )); then
  echo 'usage: scripts/check-api-compat.sh PR_BASE_REF STABLE_REF' >&2
  exit 2
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
base_ref="$1"
stable_ref="$2"
module_path='github.com/mbvlabs/andurel'
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

if ! command -v apidiff >/dev/null 2>&1; then
  echo 'apidiff is required' >&2
  exit 2
fi

cd "$repo_root"
git rev-parse --verify "${base_ref}^{commit}" >/dev/null
git rev-parse --verify "${stable_ref}^{commit}" >/dev/null

apidiff -m -w "$tmp_dir/current.export" "$module_path"

status=0
compare_ref() {
  local label="$1"
  local ref="$2"
  local ref_dir="$tmp_dir/$label"
  local report="$tmp_dir/$label.report"

  mkdir -p "$ref_dir"
  git archive "$ref" | tar -x -C "$ref_dir"
  (
    cd "$ref_dir"
    apidiff -m -w "$tmp_dir/$label.export" "$module_path"
  )
  apidiff -m -incompatible "$tmp_dir/$label.export" "$tmp_dir/current.export" > "$report"
  if [[ -s "$report" ]]; then
    echo "Incompatible public API changes relative to $ref ($label):" >&2
    cat "$report" >&2
    status=1
  fi
}

compare_ref pull-request-base "$base_ref"
if [[ "$(git rev-parse "$stable_ref")" != "$(git rev-parse "$base_ref")" ]]; then
  compare_ref stable-release "$stable_ref"
fi

exit "$status"
