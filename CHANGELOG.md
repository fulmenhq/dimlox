# Changelog

All notable changes to `dimlox` are summarized here in reverse chronological order.
This file keeps the latest 10 release entries.

## v0.1.2

First feature release after the initial public launch.

- Added multi-file `cp` with glob expansion, JSONL batch input, and positional multi-source (plan → validate → execute pipeline with `--dry-run`, `--continue-on-error`, `--max-sources`)
- Added GCS named profile support via `--gcp-profile` for all commands
- Added per-leg GCS auth for `cp` with `--gcp-profile-src/dst`, `--gcp-creds-file-src/dst`, and `--gcp-project-src/dst` for mixed-identity cloud-to-cloud copies
- Added `doctor --list-gcp-profiles` for local-only gcloud configuration inspection
- Added Homebrew and Scoop install paths with `make update-homebrew-formula` and `make update-scoop-manifest` release targets
- Aligned all exit codes with the gofulmen foundry catalog (`ExitInvalidArgument` 40, `ExitDataCorrupt` 63, `ExitResourceExhausted` 33, `ExitAuthenticationFailed` 70) — **breaking change** from v0.1.1 exit codes 2/3/4
- Added auth error classification for GCS (`ErrADCMissing`) and Azure (`AuthenticationFailedError`) sentinel errors
- Added `doctor` auth probe exit code (`ExitAuthenticationFailed` 70) for CI preflight use
- Added batch `cp` worst-failure exit code semantics under `--continue-on-error`
- Added context-cancellation cleanup for downloads (`.part` removal), uploads (temp file removal), and `cp` (landing file removal)
- Hardened test coverage for multi-file cp plan validation, glob expansion, fail-fast behavior, and cancel cleanup

## v0.1.1

Patch release for public-release readiness.

- Updated `google.golang.org/grpc` to `v1.79.3` to pick up the fix for `GHSA-p77j-4mvh-x3m3`
- Re-ran the Go quality gates and vulnerability checks before the follow-up release

## v0.1.0

Initial public release.

- Added provider-aware URI parsing for Azure Blob Storage, Google Cloud Storage, and local files
- Added `doctor` and `ls` for auth validation, connectivity checks, and target discovery
- Added streaming `get`, `put`, and `cp` workflows with progress reporting and verification options
- Added streaming `inspect` modes for delimiter detection, row counts, and head/mid/tail sampling
- Added `split` workflows for stream, range, and binary sharding with manifests, dry-run planning, and atomic writes
- Added Windows readiness work for path preflights, rename handling, PowerShell support, and native smoke coverage in CI
- Added GitHub Actions CI and release automation for quality gates, cross-builds, race checks, and draft release packaging
