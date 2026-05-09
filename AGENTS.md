# dimlox — AI Agent Guide

## Warm-Up Sequence

Read these in order before starting any task:

1. **This file** — dimlox operational protocols
2. **`AGENTS.local.md`** — machine-local overrides and coordination hub paths (gitignored; read if present)
3. **Role catalog** — `config/agentic/roles/README.md` — available roles, usage guidance, and coordination model
4. **Your role definition** — `config/agentic/roles/<your-role>.yaml`
5. **Version file** — `VERSION` — identifies the active planning folder
6. **Active milestone** — `.plans/active/v<VERSION>/README.md` — current scope and brief index (local planning file; gitignored)
7. **Active phase brief** — `.plans/active/v<VERSION>/briefs/phase-<n>-<name>.md` — authoritative phase scope (local planning file; gitignored)
8. **Implementation plan** — `.plans/dimlox-plan.md` — phase gates, interfaces, and architect review checkpoints (local planning file; gitignored)
9. **Your role state** — `.plans/roles/<your-role>/STATE.md` (if present; local planning file; gitignored)

### Session End

1. Update `.plans/roles/<your-role>/STATE.md` with current state, blockers, and next steps.
2. Follow any coordination hub instructions in `AGENTS.local.md`.

---

## Operating Model

| Aspect         | Setting                                      |
| -------------- | -------------------------------------------- |
| Mode           | Supervised (human reviews before merge)      |
| Classification | code-substantive                             |
| Role Required  | Yes                                          |
| Default Role   | devlead                                      |
| Identity       | Per session (no persistent memory)           |

---

## Roles

Role definitions live in `config/agentic/roles/`. See
[`config/agentic/roles/README.md`](config/agentic/roles/README.md) for the full
catalog, selection guide, and escalation paths.

| Role           | Focus                                              | Source                                   |
| -------------- | -------------------------------------------------- | ---------------------------------------- |
| `devlead`      | Phase implementation, provider code, CLI commands  | `config/agentic/roles/devlead.yaml`      |
| `devrev`       | Code review, phase gate compliance, correctness    | `config/agentic/roles/devrev.yaml`       |
| `qa`           | Streaming correctness, cross-provider test parity  | `config/agentic/roles/qa.yaml`           |
| `deliverylead` | AGENTS coordination, gate sequencing, handoffs     | `config/agentic/roles/deliverylead.yaml` |
| `entarch`      | Contract durability, rewrite-safe architecture     | `config/agentic/roles/entarch.yaml`      |
| `infoarch`     | Docs, plan updates, decision records               | `config/agentic/roles/infoarch.yaml`     |
| `prodmktg`     | Product positioning, devrel, adoption support      | `config/agentic/roles/prodmktg.yaml`     |
| `dataeng`      | Cloud data movement orchestration, transfer manifests, integrity validation | `config/agentic/roles/dataeng.yaml`      |

Security review is still a required function for auth/credential changes, but a
local `secrev` role file has not been added to this repo yet. Until it is,
escalate those changes directly to the human maintainer for security review.

Note: anything under `.plans/` is local planning and coordination state. It is
intentionally gitignored and is not part of the committed repository contents.

---

## PR Workflow

dimlox uses a PR-based workflow — no direct pushes to `main`.

### Branch Naming

```
<type>/<slug>-<role>-YYYYMMDD
```

Types: `feat`, `fix`, `docs`, `chore`, `test`, `refactor`, `security`

Examples:
- `feat/uri-parser-devlead-20260317`
- `feat/provider-azblob-devlead-20260318`
- `fix/stream-split-bounds-devrev-20260320`

### Review Flow

```
author (devlead) → devrev (code review)
                       ├→ human security review (if auth, credentials, or provider client code)
                       └→ human merge
```

- All PRs require at least one devrev review.
- Auth chain changes, credential handling, and provider client construction trigger
  security review by the human maintainer until a local `secrev` role file is added.
- Human always performs the merge.
- Merge strategy: **squash-merge** (default) or **rebase-merge**. Merge commits
  are disabled. Squash uses the PR title and body for the commit message.

---

## Phase Gates

Work proceeds phase by phase per `dimlox-plan.md`. Do not start the next phase
until the current gate passes architect review.

The active brief in `.plans/active/v<VERSION>/briefs/` is the phase-level source
of truth for current deliverables and acceptance criteria. If a brief and the
master plan differ, follow the active brief and raise the discrepancy.

| Phase | Deliverable              | Gate condition                                          |
| ----- | ------------------------ | ------------------------------------------------------- |
| 0     | Scaffold + URI parser    | Table-driven parse tests pass for all URI forms in §3.1 |
| 1     | Doctor + LS              | Works against real AZS and GCS endpoints                |
| 2     | Get / Put / CP           | 3.6 GB download completes without OOM                   |
| 3     | Inspect                  | Streaming wc on 594 M-row file; head/tail no full load  |
| 4     | Split                    | Shards with atomic writes; peak RSS < 500 MB            |
| 5     | Polish                   | TTY detection, dry-run, exit code audit                 |
| 6     | Windows readiness        | Cross-compile clean; split preflight + rename on Windows |

At each gate: open a PR, request devrev, and note in `.plans/roles/devlead/STATE.md`
that architect review is needed before proceeding.

---

## Quality Gates

Run before every commit:

```bash
go fmt ./...
go vet ./...
go test ./...
go build ./...
```

All four must pass. Do not skip.

---

## Commit Attribution

```
<type>(<scope>): <subject>

<body>

Changes:
- <bullet>
- <bullet>

Generated by <Model> via <Interface> under supervision of @3leapsdave

Co-Authored-By: <Model> <noreply@fulmenhq.dev>
Role: <role>
Committer-of-Record: @3leapsdave
```

> **Attribution email policy**: All AI Co-Authored-By trailers MUST use
> `noreply@fulmenhq.dev` regardless of model vendor. Do NOT use vendor addresses
> (`noreply@anthropic.com` etc.).

**Example:**

```
feat(uri): implement URI parser with provider detection

Parses AZS HTTPS, GCS HTTPS, gs://, azblob://, and local file
paths. Normalizes to internal form. Fails fast on unsupported schemes.

Changes:
- Add internal/uri/parse.go with ParseURI and Normalize
- Add table-driven tests covering all forms from dimlox-plan.md §3.1
- Return typed ParseError for unsupported schemes

Generated by Claude Sonnet via Claude Code under supervision of @3leapsdave

Co-Authored-By: Claude Sonnet <noreply@fulmenhq.dev>
Role: devlead
Committer-of-Record: @3leapsdave
```

---

## Key Files

| Path                          | Purpose                                              |
| ----------------------------- | ---------------------------------------------------- |
| `VERSION`                     | Active version / planning folder selector            |
| `.plans/active/v<VERSION>/`   | Active milestone scope and phase briefs (local planning; gitignored) |
| `.plans/dimlox-plan.md`        | Implementation plan — phase gates, interfaces, review checkpoints (local planning; gitignored) |
| `cmd/dimlox/main.go`           | CLI entry point (cobra root)                         |
| `internal/uri/parse.go`       | URI parsing and provider detection                   |
| `internal/provider/interface.go` | StorageProvider interface — the cross-cutting contract |
| `internal/provider/azblob/`   | Azure Blob Storage provider                          |
| `internal/provider/gcs/`      | Google Cloud Storage provider                        |
| `internal/provider/local/`    | Local filesystem provider                            |
| `internal/transfer/`          | Download, upload, copy, progress                     |
| `internal/split/`             | Stream-split, range-split, binary-split, manifest    |
| `internal/inspect/`           | Streaming wc, head/mid/tail                          |
| `internal/doctor/`            | Auth and connectivity checks                         |
| `config/agentic/roles/`       | Role definitions                                     |
| `.plans/roles/`               | Session-local role state (local planning; gitignored) |

---

## DO / DO NOT

### DO

- Read `.plans/dimlox-plan.md` before starting any phase
- Read files before editing them
- Run `go fmt && go vet && go test` before commits
- Keep changes focused on the current phase
- Use `bufio.NewReaderSize` — never `bufio.Scanner` for large files (see plan §8.2)
- Write all shard outputs through `.part` → atomic rename (see plan §8.1)
- Respect the `StorageProvider` interface as the only cross-cutting contract
- Use `DefaultAzureCredential` for AZS auth — do not invent alternatives
- Document provider-specific behavior differences explicitly
- Open a PR and request devrev at each phase gate

### DO NOT

- Push to `main` directly
- Skip quality gates
- Commit secrets, credentials, or access keys
- Use real client names, proprietary project names, or identifiable account names
  in code, tests, docs, or examples — use generic placeholders like `client-a`,
  `exampleaccount`, `example-project` instead. This is an OSS repo; client-specific
  identifiers must never appear in committed history.
- Load a whole file into memory in `inspect` or `split` commands
- Use `bufio.Scanner` without raising its buffer limit for large-line files
- Add dependencies without checking they are necessary
- Touch code outside the current phase scope without justification
- **EVER commit anything from `.plans/`** — gitignored, ephemeral only
- Commit `AGENTS.local.md` (gitignored)
- Hardcode account names, container names, or bucket names in non-test code
  (test code must also use generic placeholders, not real client identifiers)
- Use `git add -f` / `--force` — if a file is gitignored, fix `.gitignore` or the ignore rule; never bypass it. `git add -f` is reserved for genuine emergencies with explicit human authorisation only.

---

## Critical Rules

### Never load whole files

Every command in `inspect` and `split` must be provably streaming. Peak RSS
is a first-class correctness criterion, not a performance nice-to-have. The
machine this runs on has ~16 GB RAM and the price file is 4.1 GB uncompressed.

### Provider interface is the contract

Changes to `internal/provider/interface.go` affect all phases. Any modification
requires architect review before implementation proceeds. Do not extend it
speculatively — add only what a current phase requires.

### Auth is delegated, never owned

dimlox never stores, caches, or logs credentials. AZS auth is
`DefaultAzureCredential` + optional `--az-profile`. GCS auth is ADC +
optional `--gcp-project`. Any change to auth handling triggers human security
review until a local `secrev` role file exists.
