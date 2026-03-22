# Positioning

`dimlox` is for teams that already have cloud CLIs and shell scripts, but need a
safer way to move, inspect, and split multi-GB files without loading them into
memory.

## Primary personas

### Data Mover

This user is a data engineer, devops operator, or developer moving large CSV,
PSV, JSON, or gzip archives between Azure Blob Storage, Google Cloud Storage,
and local disk.

What they need:

- one CLI for `doctor`, `ls`, `get`, `inspect`, and `split`
- progress and exit codes that behave predictably in terminals and automation
- transfers and splits that stay memory-bounded on multi-GB files

Why `dimlox` fits:

- it keeps transfer, inspection, and sharding in one workflow instead of mixing
  provider CLIs with custom split scripts
- it uses streaming and bounded range reads instead of whole-file loads
- it documents cloud-specific tradeoffs like token lifetime risk on long stream splits

### Pipeline Tester

This user is a QA or test engineer validating data pipelines against large
reference datasets.

What they need:

- deterministic shards they can regenerate the same way every run
- manifests that make shard outputs auditable
- quick inspection of shape, row counts, and delimiters before a pipeline run

Why `dimlox` fits:

- `split --manifest` records shard metadata for every output file
- `.part` writes and atomic rename make interrupted runs obvious instead of silent
- `inspect` gives row counts and sampling without forcing a full local load

## Evidence-backed value

Current positioning claims are grounded in the recorded dogfooding notes for
Phase 2 and Phase 4.

| Workflow | Recorded evidence | What it means |
|---|---|---|
| Azure large-file download | `dimlox get` downloaded a 4.35 GiB blob in about `661.3s`; the default Azure CLI invocation used for comparison took about `1850.2s` | In this environment, `dimlox` was materially faster than the baseline Azure CLI flow that was tested |
| GCS large-file download | `dimlox get --verify` downloaded a 1.15 GiB object in about `127.0s`; `gcloud storage cp` on the same object took about `123.5s` | `dimlox` was near parity with the provider CLI on the first recorded GCS comparison |
| Large-file split | `dimlox split` processed a 4.1 GiB gzip archive into `125` shards in `10:02.18` with peak RSS around `41.3 MiB` | The split workflow stayed well below the Phase 4 memory target while producing manifest-backed shards |

## Honest boundaries

- `dimlox` does not claim to beat every provider CLI on every workload
- the strongest recorded transfer win so far is on Azure, not GCS
- same-provider server-side copy is still out of scope for `v0.1.0`
- Windows readiness is a later phase, not an MVP promise

## Why choose it over the current workflow

- use `dimlox` when the pain is not just download speed, but the whole workflow
  around a large file: auth check, transfer, inspect, split, and reproducible outputs
- use `dimlox` when you want manifests, dry runs, `.part` safety, and clear
  cloud/local mode behavior without writing glue scripts
- keep the provider CLI alone when you only need one simple provider-specific
  transfer and do not need inspect or split afterward

## Evidence trail

- `.plans/dogfooding/phase2/README.md`
- `.plans/dogfooding/phase4/README.md`

## Next docs

- [`docs/adoption/quickstart.md`](quickstart.md)
- [`docs/adoption/vs-existing.md`](vs-existing.md)
- [`docs/adoption/recipes.md`](recipes.md)
- [`docs/usage/`](../usage)
