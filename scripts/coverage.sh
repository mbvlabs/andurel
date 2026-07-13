#!/usr/bin/env bash
set -euo pipefail

readonly repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "${repo_root}"

package_list="$(go list ./...)"
packages=()
while IFS= read -r package; do
  if [[ -z "${package}" ]]; then
    continue
  fi

  case "${package}" in
    */e2e | */e2e/*) ;;
    *) packages+=("${package}") ;;
  esac
done <<< "${package_list}"

if (( ${#packages[@]} == 0 )); then
  echo "no non-e2e Go packages found" >&2
  exit 1
fi

go test "${packages[@]}" \
  -race \
  -covermode=atomic \
  -coverpkg=./... \
  -coverprofile=coverage.out
go tool cover -func=coverage.out -o coverage-summary.out
awk '/^total:/ {printf "total statement coverage: %s\n", $3; found=1} END {exit !found}' coverage-summary.out
