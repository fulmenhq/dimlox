# Role Catalog

Baseline role prompts for local AI agent sessions in `dimlox`.

These roles are tuned for a Go-based storage CLI that must solve real transfer
and split problems now while preserving interfaces for a later Rust rewrite.

## Available Roles

| Role | Slug | Category | Purpose |
|---|---|---|---|
| Development Lead | `devlead` | agentic | Implementation, architecture, code delivery |
| Development Reviewer | `devrev` | review | Code review, bug finding, four-eyes audit |
| Quality Assurance | `qa` | review | Testing, validation, quality gates |
| Delivery Lead | `deliverylead` | governance | Sprint coordination, timeline orchestration |
| Enterprise Architect | `entarch` | governance | Cross-system architecture, ecosystem parity |
| Information Architect | `infoarch` | agentic | Documentation, schema governance, standards |
| Product Marketing | `prodmktg` | agentic | Product positioning, devrel, adoption support |

## Notes

- `devlead`, `devrev`, `deliverylead`, `qa`, and `infoarch` were brought over from the
  `echoworks-mullet-transfer` shared role catalog.
- `entarch` was added here as a companion strategic architecture role because the
  source catalog references it but does not currently include a concrete role file.
- `prodmktg` was added from the `echoworks-mullet-transfer` shared catalog and tuned
  for dimlox's technical audience (dev/devops, data engineering, QA).
- The roles here are tuned to the current dimlox plan: Phase 0 scaffold, then
  provider work, transfer, inspect, and split.
- `deliverylead` is the current coordination role for local AGENTS file shaping
  and phase sequencing until a fuller routing model exists.
- Until a local `secrev` role exists, auth, credential, and secret-handling changes
  escalate directly to the human maintainer for security review.

## Suggested Use

| Work Type | Primary Role | Secondary Role |
|---|---|---|
| Scaffold and implementation | `devlead` | `devrev` |
| Architecture or contract durability | `entarch` | `devlead` |
| Test strategy or phase-gate validation | `qa` | `devrev` |
| Session sequencing and AGENTS coordination | `deliverylead` | `entarch` |
| Product positioning and adoption content | `prodmktg` | `infoarch` |
