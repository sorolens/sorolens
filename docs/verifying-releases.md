# Verifying releases

Every release artifact can be traced back to the workflow run in this
repository that built it:

- **Container images** (`ghcr.io/sorolens/sorolens-api`, `ghcr.io/sorolens/sorolens-indexer`)
  are signed with [cosign](https://docs.sigstore.dev/cosign/signing/overview/) and carry
  a SLSA build provenance attestation. BuildKit also attaches an SPDX SBOM and
  its own provenance to the image index.
- **CLI binaries** (`cli-v*` releases) ship a `checksums.txt` that is signed
  with cosign (`checksums.txt.sigstore.json`). Each archive has a SLSA build
  provenance attestation and an SBOM (`*.sbom.json`).

Signing is keyless. No signing key exists: each signature is bound to the
GitHub Actions OIDC identity of the release workflow that produced it and
recorded in the public [Rekor](https://docs.sigstore.dev/logging/overview/)
transparency log. Verifying a signature therefore checks two things: that the
artifact is unchanged, and that it was built by `release.yml` or
`cli-release.yml` in `sorolens/sorolens` from a release tag.

You need [cosign](https://docs.sigstore.dev/cosign/system_config/installation/)
v2 or later and, for provenance, the [GitHub CLI](https://cli.github.com/).
The examples use `v1.2.3` and `cli-v1.2.3`; substitute the release you are
checking.

## Container images

Verify the signature. The identity must be the release workflow, run from a
`v*` tag:

```bash
cosign verify ghcr.io/sorolens/sorolens-api:1.2.3 \
  --certificate-identity-regexp '^https://github\.com/sorolens/sorolens/\.github/workflows/release\.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

Verify the build provenance:

```bash
gh attestation verify oci://ghcr.io/sorolens/sorolens-api:1.2.3 \
  --repo sorolens/sorolens \
  --signer-workflow sorolens/sorolens/.github/workflows/release.yml
```

Repeat both for `sorolens-indexer`. A tag can be moved to a new image, so for
deployments pin the digest that verified (`ghcr.io/sorolens/sorolens-api@sha256:...`).

To read the SBOM BuildKit attached to the image:

```bash
docker buildx imagetools inspect ghcr.io/sorolens/sorolens-api:1.2.3 \
  --format '{{ json .SBOM }}'
```

## CLI binaries

Download the archive for your platform, `checksums.txt` and
`checksums.txt.sigstore.json` from the release page, for example:

```bash
gh release download cli-v1.2.3 --repo sorolens/sorolens \
  --pattern 'sorolens_1.2.3_linux_amd64.tar.gz' \
  --pattern 'checksums.txt' \
  --pattern 'checksums.txt.sigstore.json'
```

Verify the signature over the checksums, which proves `checksums.txt` came
from the CLI release workflow:

```bash
cosign verify-blob checksums.txt \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github\.com/sorolens/sorolens/\.github/workflows/cli-release\.yml@refs/tags/cli-v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

Then check the archive against it:

```bash
sha256sum --ignore-missing -c checksums.txt   # macOS: shasum -a 256 --ignore-missing -c checksums.txt
```

Verify the archive's build provenance:

```bash
gh attestation verify sorolens_1.2.3_linux_amd64.tar.gz \
  --repo sorolens/sorolens \
  --signer-workflow sorolens/sorolens/.github/workflows/cli-release.yml
```

The Homebrew cask installs the same archives, so a binary installed with
`brew install sorolens/tap/sorolens` can be checked by downloading the matching
archive and comparing checksums.

## What a failure means

- **`no matching signatures` / identity mismatch**: the artifact was not signed
  by the release workflow from a release tag. Do not run it.
- **checksum `FAILED`**: the archive differs from the one the release built.
  Download it again from the release page; if it still fails, report it.
- **`gh attestation verify` finds no attestation**: releases published before
  signing was added have none. Every release from the first one that ships
  `checksums.txt.sigstore.json` onwards should verify.

## How it is produced

- [`release.yml`](https://github.com/sorolens/sorolens/blob/main/.github/workflows/release.yml) builds each image with
  `sbom: true` and `provenance: mode=max`, signs the pushed digest with
  `cosign sign`, and runs `actions/attest-build-provenance` with
  `push-to-registry: true`.
- [`cli-release.yml`](https://github.com/sorolens/sorolens/blob/main/.github/workflows/cli-release.yml) runs GoReleaser,
  whose `signs` section in [`cli/.goreleaser.yml`](https://github.com/sorolens/sorolens/blob/main/cli/.goreleaser.yml)
  signs `checksums.txt` with `cosign sign-blob`, then attests the archives and
  the checksums file. The manual snapshot run skips signing, because it has no
  release identity.

Both run only on release tags; no pull request check signs or attests anything.
