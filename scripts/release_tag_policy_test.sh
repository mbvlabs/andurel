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
git -C "${root}" tag v1.1.0-beta.1
expect_pass "${root}" v1.1.0-beta.1 "${sha}"
git -C "${root}" tag -a v1.1.0-beta.2 -m beta2
expect_pass "${root}" v1.1.0-beta.2 "${sha}"

root="$(new_repository stable-without-prerelease)"
sha="$(git -C "${root}" rev-parse HEAD)"
git -C "${root}" tag v1.0.0
expect_pass "${root}" v1.0.0 "${sha}"

root="$(new_repository independent-prerelease)"
git -C "${root}" tag v1.0.0-beta.1
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
for job in tag-identity draft-release publish; do
  block="$(sed -n "/^  ${job}:/,/^  [a-zA-Z0-9-]*:/p" "${workflow}")"
  if [[ "${block}" != *'./scripts/verify-release-tag.sh "${GITHUB_REF_NAME}" "${GITHUB_SHA}"'* ]]; then
    echo "${job} does not invoke the centralized release tag policy" >&2
    exit 1
  fi
done

readiness_block="$(sed -n '/^  readiness:/,/^  [a-zA-Z0-9-]*:/p' "${workflow}")"
if [[ "${readiness_block}" != *'uses: ./.github/workflows/release-readiness.yml'* ]]; then
  echo 'release workflow does not invoke canonical release readiness' >&2
  exit 1
fi

preflight_block="$(sed -n '/^  preflight:/,/^  [a-zA-Z0-9-]*:/p' "${workflow}")"
for dependency in tag-identity readiness; do
  if [[ "${preflight_block}" != *"- ${dependency}"* ]]; then
    echo "release artifact preflight does not require ${dependency}" >&2
    exit 1
  fi
done

readiness_workflow="${repo_root}/.github/workflows/release-readiness.yml"
for trigger in pull_request push workflow_call; do
  if ! grep -Eq "^  ${trigger}:" "${readiness_workflow}"; then
    echo "canonical release readiness is missing ${trigger}" >&2
    exit 1
  fi
done
if ! grep -Fq '      - master' "${readiness_workflow}"; then
  echo 'canonical release readiness does not target master' >&2
  exit 1
fi

echo "release tag policy contract passed"
