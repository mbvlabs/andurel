#!/usr/bin/env bash
set -euo pipefail

if (( $# < 3 || $# > 4 )); then
  echo 'usage: scripts/smoke-release-archive.sh ARCHIVE TAG ASSET_DIRECTORY [SIGNING_IDENTITY]' >&2
  exit 2
fi

archive="$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
tag="$2"
asset_dir="$(cd "$3" && pwd)"
version="${tag#v}"
archive_name="$(basename "${archive}")"
checksum_file="${asset_dir}/checksums.txt"
sbom_checksum_file="${asset_dir}/sbom-checksums.txt"
sbom="${asset_dir}/${archive_name}.sbom.json"
signing_identity="${4:-https://github.com/${GITHUB_REPOSITORY:-mbvlabs/andurel}/.github/workflows/release.yml@refs/tags/${tag}}"

case "$(uname -s)" in
  Linux) expected_os=linux ;;
  Darwin) expected_os=darwin ;;
  *) echo "unsupported smoke-test operating system: $(uname -s)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  x86_64|amd64) expected_arch=amd64 ;;
  arm64|aarch64) expected_arch=arm64 ;;
  *) echo "unsupported smoke-test architecture: $(uname -m)" >&2; exit 1 ;;
esac

expected_name="andurel_${version}_${expected_os}_${expected_arch}.tar.gz"
test "${archive_name}" = "${expected_name}"
test -f "${archive}"
test -f "${sbom}"
test -f "${checksum_file}"
test -f "${checksum_file}.sigstore.json"
test -f "${sbom_checksum_file}"
test -f "${sbom_checksum_file}.sigstore.json"

cosign verify-blob \
  --bundle "${checksum_file}.sigstore.json" \
  --certificate-identity "${signing_identity}" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  "${checksum_file}"
cosign verify-blob \
  --bundle "${sbom_checksum_file}.sigstore.json" \
  --certificate-identity "${signing_identity}" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  "${sbom_checksum_file}"

verify_manifest_entry() {
  local manifest="$1"
  local file="$2"
  local name
  local expected
  local actual

  name="$(basename "${file}")"
  expected="$(awk -v name="${name}" '$2 == name {print $1}' "${manifest}")"
  test -n "${expected}"
  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "${file}" | awk '{print $1}')"
  else
    actual="$(shasum -a 256 "${file}" | awk '{print $1}')"
  fi
  test "${actual}" = "${expected}"
}

verify_manifest_entry "${checksum_file}" "${archive}"
verify_manifest_entry "${sbom_checksum_file}" "${sbom}"
jq -e '.spdxVersion | startswith("SPDX-")' "${sbom}" >/dev/null

tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/andurel-release-smoke.XXXXXX")"
cleanup() {
  rm -rf "${tmp_dir}"
}
trap cleanup EXIT INT TERM

install_dir="${tmp_dir}/install"
mkdir -p "${install_dir}"
tar -xzf "${archive}" -C "${install_dir}"
binary="${install_dir}/andurel"
test -x "${binary}"
"${binary}" --version | grep -F "${version}" >/dev/null
"${binary}" commands --json > "${tmp_dir}/commands.json"
jq -e '.ok == true and (.data.commands | type == "array")' "${tmp_dir}/commands.json" >/dev/null
