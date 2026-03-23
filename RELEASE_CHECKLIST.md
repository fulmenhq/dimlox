# Release Checklist

Standard checklist for dimlox releases.

## Pre-Release

### Scope and quality

- [ ] Current phase gate is complete and ready for release
- [ ] Planned changes are implemented and reviewed
- [ ] `make fmt` passes
- [ ] `make vet` passes
- [ ] `make test` passes
- [ ] `make build` passes
- [ ] `make build-all` passes
- [ ] README and relevant docs reflect the release behavior

### Version prep

- [ ] Update `VERSION`: `make version-set V=vX.Y.Z`
- [ ] Search for stale hardcoded version references
- [ ] Commit all release-prep changes
- [ ] Confirm `git status` is clean

## CI Release Build

### Tag and push

- [ ] Create annotated tag: `git tag -a vX.Y.Z -m "Release vX.Y.Z"`
- [ ] Verify tag locally: `git tag -v vX.Y.Z`
- [ ] Push `main`: `git push origin main`
- [ ] Push tag: `git push origin vX.Y.Z`
- [ ] Confirm the GitHub release workflow creates a draft release with CI-built artifacts

## Local Signing and Provenance

Set signing values outside the repo using whatever local storage pattern fits your maintainer setup, then export the release tag for this run:

| Variable | Purpose |
|---|---|
| `DIMLOX_GPG_HOMEDIR` | GnuPG homedir for signing |
| `DIMLOX_PGP_KEY_ID` | Optional PGP signing key fingerprint |
| `DIMLOX_MINISIGN_KEY` | Minisign private key path |
| `DIMLOX_MINISIGN_PUB` | Minisign public key path |
| `DIMLOX_RELEASE_TAG` | Release tag for this release |

```bash
# export or source your local signing variables first
export DIMLOX_RELEASE_TAG=vX.Y.Z
```

### Preferred path: sign CI-built artifacts

- [ ] `make release-clean`
- [ ] `make release-download`
- [ ] `make release-checksums`
- [ ] `make release-verify-checksums`
- [ ] `make release-sign`
- [ ] `make release-export-keys`
- [ ] `make release-verify-keys`
- [ ] Optional notes: `make release-notes`
- [ ] Upload provenance: `make release-upload`

### Fallback path: build locally if CI artifacts are unavailable

- [ ] `make release-build`
- [ ] `make release-verify-checksums`
- [ ] `make release-sign`
- [ ] `make release-export-keys`
- [ ] `make release-verify-keys`
- [ ] Optional notes: `make release-notes`
- [ ] Upload all assets manually: `make release-upload-all`

## Publish

- [ ] Review the draft GitHub Release
- [ ] Verify binaries, manifests, signatures, and public keys are present
- [ ] Publish the release

## Post-Release

- [ ] Verify `go install github.com/fulmenhq/dimlox/cmd/dimlox@vX.Y.Z` works
- [ ] Verify release notes and usage docs match the published behavior
- [ ] Monitor for immediate regressions or credential/setup issues
