#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 2 ]]; then
  echo "usage: $0 <tag> <expected-sha>" >&2
  exit 2
fi

tag="$1"
expected_sha="$2"

if [[ ! "${tag}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  echo "release tag ${tag} does not match the required semantic tag pattern" >&2
  exit 1
fi
if [[ ! "${expected_sha}" =~ ^[0-9a-f]{40}$ ]]; then
  echo "expected SHA must be a full lowercase 40-character commit ID" >&2
  exit 1
fi

release_ref="refs/tags/${tag}^{commit}"
if ! release_commit="$(git rev-parse --verify "${release_ref}" 2>/dev/null)"; then
  echo "release tag ${tag} does not exist or does not peel to a commit" >&2
  exit 1
fi
head_commit="$(git rev-parse --verify HEAD)"
if [[ "${release_commit}" != "${expected_sha}" ]]; then
  echo "release tag ${tag} peels to ${release_commit}, expected ${expected_sha}" >&2
  exit 1
fi
if [[ "${head_commit}" != "${expected_sha}" ]]; then
  echo "HEAD is ${head_commit}, expected release commit ${expected_sha}" >&2
  exit 1
fi
