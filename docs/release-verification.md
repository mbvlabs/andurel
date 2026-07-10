# Verifying Andurel releases

Official releases contain four archives:

| Operating system | Architecture | Archive suffix |
| --- | --- | --- |
| Linux | amd64 | `linux_amd64.tar.gz` |
| Linux | arm64 | `linux_arm64.tar.gz` |
| macOS | amd64 | `darwin_amd64.tar.gz` |
| macOS | arm64 | `darwin_arm64.tar.gz` |

Windows is not supported in Andurel v1. Each archive has a matching `.sbom.json` SPDX SBOM. The release also contains `checksums.txt`, `sbom-checksums.txt`, and a keyless Sigstore bundle for each manifest.

The release workflow installs and executes every archive on its native operating system and architecture before making the draft public. It also publishes GitHub artifact attestations for every archive and SBOM.

## Download

Set the version without the leading `v`, then download one archive and the verification metadata:

```bash
VERSION=1.0.0
TAG="v${VERSION}"
ARCHIVE="andurel_${VERSION}_linux_amd64.tar.gz"

gh release download "${TAG}" --repo mbvlabs/andurel \
  --pattern "${ARCHIVE}" \
  --pattern "${ARCHIVE}.sbom.json" \
  --pattern checksums.txt \
  --pattern checksums.txt.sigstore.json \
  --pattern sbom-checksums.txt \
  --pattern sbom-checksums.txt.sigstore.json
```

Use `darwin` for macOS and select `amd64` or `arm64` to match `uname -m`.

## Verify the keyless signatures

Install Cosign 3, then verify both signed manifests. The certificate identity binds the signature to this repository's tag workflow:

```bash
IDENTITY="https://github.com/mbvlabs/andurel/.github/workflows/release.yml@refs/tags/${TAG}"
ISSUER="https://token.actions.githubusercontent.com"

cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity "${IDENTITY}" \
  --certificate-oidc-issuer "${ISSUER}" \
  checksums.txt

cosign verify-blob \
  --bundle sbom-checksums.txt.sigstore.json \
  --certificate-identity "${IDENTITY}" \
  --certificate-oidc-issuer "${ISSUER}" \
  sbom-checksums.txt
```

Do not continue if either identity, issuer, or signature check fails.

## Verify checksums and SBOM

On Linux:

```bash
grep "  ${ARCHIVE}$" checksums.txt | sha256sum --check
grep "  ${ARCHIVE}.sbom.json$" sbom-checksums.txt | sha256sum --check
```

On macOS:

```bash
grep "  ${ARCHIVE}$" checksums.txt | shasum -a 256 --check
grep "  ${ARCHIVE}.sbom.json$" sbom-checksums.txt | shasum -a 256 --check
```

The SBOM uses SPDX JSON and can be inspected with:

```bash
jq '.spdxVersion, .packages[]?.name' "${ARCHIVE}.sbom.json"
```

## Verify build provenance

GitHub attestations bind each artifact digest to the tag workflow run:

```bash
gh attestation verify "${ARCHIVE}" --repo mbvlabs/andurel
gh attestation verify "${ARCHIVE}.sbom.json" --repo mbvlabs/andurel
```

## Install the verified archive

```bash
mkdir -p "${HOME}/.local/bin"
tar -xzf "${ARCHIVE}" -C "${HOME}/.local/bin" andurel
"${HOME}/.local/bin/andurel" --version
```

Ensure `${HOME}/.local/bin` is on `PATH`. The printed version must match `${VERSION}`.
