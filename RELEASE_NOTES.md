# Release Notes

This file keeps the latest 3 release summaries in reverse chronological order.

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
