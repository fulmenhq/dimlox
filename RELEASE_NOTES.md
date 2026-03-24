# Release Notes

This file keeps the latest 3 release summaries in reverse chronological order.

## v0.1.1

`dimlox` 0.1.1 is a focused patch release that updates the gRPC dependency used
in the module graph so the public release line starts from the fixed advisory
version.

Highlights:

- Updates `google.golang.org/grpc` from `v1.79.2` to `v1.79.3`
- Clears the `GHSA-p77j-4mvh-x3m3` advisory from the active module selection
- Keeps CLI behavior unchanged while improving public-release readiness

See `docs/releases/v0.1.1.md` for the full release notes.

## v0.1.0

`dimlox` 0.1.0 is the first public release of the CLI for moving, inspecting,
and splitting large files across Azure Blob Storage, Google Cloud Storage, and
local filesystems without loading whole files into memory.

Highlights:

- Streaming transfers for `get`, `put`, and `cp` with progress reporting and checksum verification options
- Inspection commands for delimiter and encoding detection, row counts, and targeted head/mid/tail sampling
- Repeatable split workflows with manifests, dry-run planning, atomic shard writes, and Windows path preflight checks
- Provider health and listing workflows through `doctor` and `ls`
- Windows-aware developer workflows plus CI, cross-build, and draft release automation

See `docs/releases/v0.1.0.md` for the full release notes.
