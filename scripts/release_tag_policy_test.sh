#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
verify_script="${repo_root}/scripts/verify-release-tag.sh"
temporary_root="$(mktemp -d)"
trap 'rm -rf "${temporary_root}"' EXIT

new_repository() {
  local name="$1"
  local root="${temporary_root}/${name}"
  git init -q "${root}"
  git -C "${root}" config user.email release-policy@example.com
  git -C "${root}" config user.name "Release Policy"
  git -C "${root}" commit -q --allow-empty -m first
  printf '%s\n' "${root}"
}

expect_pass() {
  local root="$1"
  local tag="$2"
  local sha="$3"
  if ! (cd "${root}" && "${verify_script}" "${tag}" "${sha}"); then
    echo "expected ${tag} policy check to pass" >&2
    exit 1
  fi
}

expect_fail() {
  local root="$1"
  local tag="$2"
  local sha="$3"
  if (cd "${root}" && "${verify_script}" "${tag}" "${sha}" >/dev/null 2>&1); then
    echo "expected ${tag} policy check to fail" >&2
    exit 1
  fi
}

root="$(new_repository prerelease)"
sha="$(git -C "${root}" rev-parse HEAD)"
git -C "${root}" tag v1.0.0-rc.5
expect_pass "${root}" v1.0.0-rc.5 "${sha}"
git -C "${root}" tag -a v1.0.0-rc.6 -m rc6
expect_pass "${root}" v1.0.0-rc.6 "${sha}"

root="$(new_repository stable-without-rc)"
sha="$(git -C "${root}" rev-parse HEAD)"
git -C "${root}" tag v1.0.0
expect_pass "${root}" v1.0.0 "${sha}"

root="$(new_repository independent-rc)"
git -C "${root}" tag v1.0.0-rc.4
git -C "${root}" commit -q --allow-empty -m second
sha="$(git -C "${root}" rev-parse HEAD)"
git -C "${root}" tag -a v1.0.0 -m stable
expect_pass "${root}" v1.0.0 "${sha}"

root="$(new_repository mismatches)"
sha="$(git -C "${root}" rev-parse HEAD)"
git -C "${root}" tag v2.0.0
git -C "${root}" commit -q --allow-empty -m second
head_sha="$(git -C "${root}" rev-parse HEAD)"
expect_fail "${root}" v2.0.0 "${head_sha}"
expect_fail "${root}" v2.0.0 "${sha}"
expect_fail "${root}" release-2.0.0 "${head_sha}"

workflow="${repo_root}/.github/workflows/release.yml"
if ! grep -Fq 'GORELEASER_CURRENT_TAG: ${{ github.ref_name }}' "${workflow}"; then
  echo "release workflow does not pin GoReleaser to the triggering tag" >&2
  exit 1
fi
for job in validate draft-release publish; do
  block="$(sed -n "/^  ${job}:/,/^  [a-zA-Z0-9-]*:/p" "${workflow}")"
  if [[ "${block}" != *'./scripts/verify-release-tag.sh "${GITHUB_REF_NAME}" "${GITHUB_SHA}"'* ]]; then
    echo "${job} does not invoke the centralized release tag policy" >&2
    exit 1
  fi
done

echo "release tag policy contract passed"
