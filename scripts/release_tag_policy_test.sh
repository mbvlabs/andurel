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

tag_identity_block="$(sed -n '/^  tag-identity:/,/^  [a-zA-Z0-9-]*:/p' "${workflow}")"
for requirement in \
  'actions: read' \
  'gh run list' \
  '--workflow release-readiness.yml' \
  '--branch master' \
  '--commit "${GITHUB_SHA}"' \
  '--event push' \
  '--status success'; do
  if [[ "${tag_identity_block}" != *"${requirement}"* ]]; then
    echo "release workflow does not verify ${requirement}" >&2
    exit 1
  fi
done

preflight_block="$(sed -n '/^  artifact-preflight:/,/^  [a-zA-Z0-9-]*:/p' "${workflow}")"
for requirement in \
  'needs: tag-identity' \
  'uses: ./.github/workflows/release-artifact-preflight.yml' \
  'checkout_ref: ${{ github.sha }}' \
  'release_tag: ${{ github.ref_name }}'; do
  if [[ "${preflight_block}" != *"${requirement}"* ]]; then
    echo "release artifact preflight does not enforce ${requirement}" >&2
    exit 1
  fi
done
if [[ "${preflight_block}" == *'signing_identity:'* ]]; then
  echo 'release artifact preflight must derive its signing identity internally' >&2
  exit 1
fi

readiness_workflow="${repo_root}/.github/workflows/release-readiness.yml"
if ! grep -Eq '^  push:' "${readiness_workflow}"; then
  echo 'canonical release readiness is missing push' >&2
  exit 1
fi
if grep -Eq '^  (pull_request|workflow_call):' "${readiness_workflow}"; then
  echo 'canonical release readiness must run only after merge' >&2
  exit 1
fi
if ! grep -Fq '      - master' "${readiness_workflow}"; then
  echo 'canonical release readiness does not target master' >&2
  exit 1
fi

readiness_preflight_block="$(sed -n '/^  artifact-preflight:/,/^  [a-zA-Z0-9-]*:/p' "${readiness_workflow}")"
for requirement in \
  'needs: readiness' \
  'uses: ./.github/workflows/release-artifact-preflight.yml' \
  'checkout_ref: ${{ github.sha }}' \
  "release_tag: \${{ format('v0.0.0-readiness.{0}', github.run_id) }}"; do
  if [[ "${readiness_preflight_block}" != *"${requirement}"* ]]; then
    echo "canonical release readiness artifact preflight does not enforce ${requirement}" >&2
    exit 1
  fi
done
if [[ "${readiness_preflight_block}" == *'signing_identity:'* ]]; then
  echo 'release readiness preflight must derive its signing identity internally' >&2
  exit 1
fi

artifact_preflight_workflow="${repo_root}/.github/workflows/release-artifact-preflight.yml"
for requirement in \
  'workflow_call:' \
  'name: Prepare exact preflight tag' \
  'git show-ref --verify --quiet "refs/tags/${PREFLIGHT_TAG}"' \
  'git tag -- "${PREFLIGHT_TAG}" "${head_commit}"' \
  'tag_commit="$(git rev-list -n 1 "${PREFLIGHT_TAG}")"' \
  "SIGNING_IDENTITY: \${{ format('https://github.com/{0}/.github/workflows/release-artifact-preflight.yml@{1}', github.repository, github.ref) }}" \
  'args: release --clean --skip=publish' \
  'linux_amd64 linux_arm64 darwin_amd64 darwin_arm64' \
  'ubuntu-24.04-arm' \
  'macos-15-intel' \
  'macos-15' \
  './scripts/smoke-release-archive.sh'; do
  if ! grep -Fq "${requirement}" "${artifact_preflight_workflow}"; then
    echo "shared release artifact preflight is missing ${requirement}" >&2
    exit 1
  fi
done

test_workflow="${repo_root}/.github/workflows/test.yml"
for requirement in \
  'pull_request:' \
  '      - master' \
  '  test:' \
  'go vet ./...' \
  'go test $(go list ./... | grep -v /e2e)' \
  'go test ./e2e/... -v -timeout 25m'; do
  if ! grep -Fq "${requirement}" "${test_workflow}"; then
    echo "pull request test workflow is missing ${requirement}" >&2
    exit 1
  fi
done

coverage_workflow="${repo_root}/.github/workflows/coverage.yml"
if grep -Eq '^  release:' "${coverage_workflow}" || \
  ! grep -Eq '^  workflow_run:' "${coverage_workflow}" || \
  ! grep -Fq '      - Release' "${coverage_workflow}" || \
  ! grep -Fq "github.event.workflow_run.conclusion == 'success'" "${coverage_workflow}"; then
  echo 'coverage workflow must run after a successful Release workflow completion' >&2
  exit 1
fi

echo "release tag policy contract passed"
