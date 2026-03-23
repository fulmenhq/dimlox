# Changelog

All notable changes to `dimlox` are summarized here in reverse chronological order.
This file keeps the latest 10 release entries.

## v0.1.0

Initial public release.

- Added provider-aware URI parsing for Azure Blob Storage, Google Cloud Storage, and local files
- Added `doctor` and `ls` for auth validation, connectivity checks, and target discovery
- Added streaming `get`, `put`, and `cp` workflows with progress reporting and verification options
- Added streaming `inspect` modes for delimiter detection, row counts, and head/mid/tail sampling
- Added `split` workflows for stream, range, and binary sharding with manifests, dry-run planning, and atomic writes
- Added Windows readiness work for path preflights, rename handling, PowerShell support, and native smoke coverage in CI
- Added GitHub Actions CI and release automation for quality gates, cross-builds, race checks, and draft release packaging
