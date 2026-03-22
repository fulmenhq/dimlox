# ADR-0001: Cross-Platform Compatibility Requirements

**Status:** Accepted
**Date:** 2026-03-22
**Deciders:** @3leapsdave (maintainer), deliverylead, entarch
**Phase:** 6 (Windows readiness)

## Context

dimlox targets dev/devops engineers, data engineers, and QA personnel who work
across Linux, macOS, and Windows. Azure Blob Storage usage is particularly
common in Windows-heavy enterprise environments. Phases 0-5 were developed and
tested on Linux (arm64); Phase 6 extends the build and runtime contract to
Windows and formalises requirements that apply to all three platforms.

This ADR establishes the cross-platform rules that all future code must follow.
It is not retroactive documentation of existing behaviour — it is a forward
contract that Phase 6 implements and all subsequent phases must respect.

## Decision

### 1. Build contract

All releases must cross-compile cleanly for the following targets:

| GOOS    | GOARCH | Priority |
|---------|--------|----------|
| linux   | amd64  | primary  |
| linux   | arm64  | primary  |
| darwin  | amd64  | primary  |
| darwin  | arm64  | primary  |
| windows | amd64  | primary  |
| windows | arm64  | secondary |

`make check` (or a dedicated `make build-all`) must include
`GOOS=windows GOARCH=amd64 go build ./...` at minimum. Failing this gate
blocks merge.

Platform-specific code must use build tags (`//go:build windows`, etc.) and
must have a corresponding implementation or stub for all primary targets.
No `_linux.go` file without a `_windows.go` and `_darwin.go` counterpart
where the behaviour differs.

### 2. Path separators

Two path domains exist in dimlox and must never be conflated:

| Domain | Separator | When to use |
|--------|-----------|-------------|
| **OS-native** | `filepath.Join`, `filepath.Separator` | Local file I/O: opening, creating, renaming, and stating files |
| **Portable** | Forward slash (`/`) always | URI paths, manifest entries (`shard_path`, `source_uri`), cloud object keys, CLI output for object listings |

**Rules:**

- Use `filepath.Join` for any path that will be passed to `os.Open`,
  `os.Create`, `os.Rename`, or similar OS calls.
- Use `path.Join` or explicit `/` for any path that appears in manifests,
  JSON output, URI construction, or cloud provider API calls.
- Never compare an OS-native path to a portable path without normalising first.
- Manifest files are interchange artifacts — a manifest written on Windows must
  be readable on Linux and vice versa. Therefore manifest shard paths are always
  forward-slash, relative to `out_dir`.
- Local provider recursive `ls` output uses forward slashes (already implemented)
  to maintain cloud-key parity.

### 3. Path length and filename validation

**Preflight check:** Before any multi-file write operation (split, batch copy),
compute the longest planned output path and validate it:

| Platform | Limit | Behaviour |
|----------|-------|-----------|
| Windows (`runtime.GOOS == "windows"`) | 260 chars (`MAX_PATH`) | Error with actionable message suggesting shorter `--out-dir` |
| Linux, macOS | 260 chars | Warn to stderr (NFS, SMB, and some container runtimes also enforce MAX_PATH) |

The check must run in both real and `--dry-run` modes so users can preflight
before committing to I/O.

**Filename character validation:** When the source filename stem contains
characters illegal on Windows (`< > : " / \ | ? *`), the split shard namer
must sanitise or reject before writing. This applies on all platforms — a
manifest produced on Linux should name shards that Windows can consume.

### 4. Atomic file operations

The `.part` → final rename pattern is a core safety contract in split.

| Platform | `os.Rename` behaviour | Required handling |
|----------|----------------------|-------------------|
| Linux, macOS | Overwrites target atomically | No extra handling needed |
| Windows | Fails if target exists | Remove target before rename; handle `ACCESS_DENIED` if target is locked by another process |

Implementation must use a helper function (e.g. `atomicRename(src, dst)`)
that encapsulates the platform difference. Do not scatter `runtime.GOOS`
checks at each rename call site.

### 5. Terminal and TTY detection

Progress output must degrade gracefully:

| Environment | Behaviour |
|-------------|-----------|
| TTY with ANSI support (Linux/macOS terminals, Windows Terminal, recent PowerShell) | ANSI progress bars and colours |
| TTY without ANSI support (cmd.exe, older PowerShell without VT processing) | Plain text progress (no escape sequences) |
| Piped / non-TTY | Structured JSON lines to stderr, no ANSI, no progress bars |

**Rules:**

- Use `golang.org/x/term.IsTerminal()` or equivalent for TTY detection —
  it handles Windows console handles correctly.
- On Windows, do not assume ANSI support just because output is a TTY.
  Either probe for virtual terminal processing capability or default to
  plain text on Windows TTYs unless `TERM` or `WT_SESSION` indicates
  modern terminal support.
- Never emit ANSI escapes to a non-TTY fd regardless of platform.

### 6. Signal handling and cleanup

Ctrl+C cleanup (removing `.part` files) must work on all platforms:

| Platform | Signal | Notes |
|----------|--------|-------|
| Linux, macOS | `os.Interrupt`, `syscall.SIGTERM` | Both must trigger cleanup |
| Windows | `os.Interrupt` only | `syscall.SIGTERM` does not exist; do not reference it unconditionally |

**Rules:**

- Signal registration must use build tags or runtime checks to avoid
  referencing `syscall.SIGTERM` on Windows.
- Cleanup must be idempotent — safe to call on partial state if the
  interrupt arrives mid-write.
- `context.WithCancel` propagation is preferred over direct signal handling
  in library code; only the top-level CLI entry point should register
  signal handlers.

### 7. Temporary directories and default paths

- Use `os.TempDir()` for ephemeral files — never hardcode `/tmp`.
- `DIMLOX_LANDING_DIR` must resolve correctly on all platforms. If unset,
  fall back to `os.TempDir()`, not a POSIX-specific path.
- Install paths: `~/.local/bin/dimlox` on Linux/macOS,
  `%APPDATA%\dimlox\bin\dimlox.exe` on Windows, as documented in README.

## Consequences

- All new code touching file paths, terminal output, signal handling, or
  temp directories must follow this ADR. Code review (devrev) should check
  compliance.
- Phase 6 implements these requirements against the existing codebase.
  Subsequent phases inherit them as standing constraints.
- The `atomicRename` helper and path-length preflight become shared
  utilities that other commands (not just split) can use if they write
  multiple output files.
- Manifest portability is a new explicit contract — once Phase 6 ships,
  manifests are a cross-platform interchange format.

## Alternatives Considered

- **Windows-specific binary with separate build**: Rejected. Go's
  cross-compilation and build tags handle this cleanly without maintaining
  separate codebases.
- **Ignore Windows until v0.2.0**: Rejected. The audience overlap with
  Azure enterprise environments makes Windows support high-value for
  adoption, and the required changes are bounded.
- **Use `\\?\` long-path prefix on Windows to bypass MAX_PATH**: Deferred.
  This introduces complexity in path display and logging. Preflight
  checking with a clear error is simpler and covers NFS/SMB too.
