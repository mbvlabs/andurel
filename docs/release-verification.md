# Verifying Andurel releases

Official releases contain four archives:

| Operating system | Architecture | Archive suffix |
| --- | --- | --- |
| Linux | amd64 | `linux_amd64.tar.gz` |
| Linux | arm64 | `linux_arm64.tar.gz` |
| macOS | amd64 | `darwin_amd64.tar.gz` |
| macOS | arm64 | `darwin_arm64.tar.gz` |

Windows is not supported in Andurel v2. Each archive has a matching `.sbom.json` SPDX SBOM. The release also contains `checksums.txt`, `sbom-checksums.txt`, and a keyless Sigstore bundle for each manifest.

The release workflow installs and executes every archive on its native operating system and architecture before making the draft public. It also publishes GitHub artifact attestations for every archive and SBOM.

## Download

Set the version without the leading `v`, then download one archive and the verification metadata:

```bash
VERSION=2.0.0-alpha.5
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

## Verify the SSR production story

In a scaffolded Inertia project, confirm both SSR transports before releasing:

1. **Development SSR through Vite.** Opt a page into SSR (`.SSR()` on its
   page builder), run `andurel run`, and load that page. The response must
   contain `data-server-rendered="true"`; Vite's `/__inertia_ssr` endpoint
   serves it. This catches the dev-server regression that an unconditional
   `ssr.noExternal` build setting causes.
2. **Production SSR from the embedded bundle.** Run `pnpm build` and build
   `cmd/ssr`, then start the binary from an empty directory (with
   `INERTIA_SSR_BUNDLE` unset or pointing at a missing absolute path):

   ```bash
   go build -o /tmp/ssr-check ./cmd/ssr
   (cd "$(mktemp -d)" && /tmp/ssr-check)
   ```

   The process must stay up, report its listen address, and answer
   `GET /health` with `{"status":"ok"}`. A failure to start here means the
   embedded bundle is missing or still has bare imports.
