# dimlox vs Existing Workflows

`dimlox` is not trying to replace every provider CLI command. It is for the
cases where a large-file workflow turns into a chain of downloads, checks,
inspection steps, and custom split scripts.

## Comparison table

| Baseline workflow | Where `dimlox` helps | Recorded evidence | Stay with the baseline when |
|---|---|---|---|
| `az storage blob download` for a large Azure object | Keeps the transfer in the same CLI you will later use for `inspect` and `split`; gives consistent progress and exit-code behavior | On a recorded 4.35 GiB Azure blob, `dimlox get` finished in about `661.3s` vs about `1850.2s` for the default Azure CLI invocation that was tested | You only need a one-off Azure download and already have a tuned Azure-only flow |
| `gcloud storage cp` for a large GCS object | Adds `--verify`, shared docs, and the follow-on inspect/split workflow in the same tool | On a recorded 1.15 GiB GCS object, `dimlox get --verify` finished in about `127.0s` vs about `123.5s` for `gcloud storage cp` | You are GCS-only and do not need inspect or split after the copy |
| Hand-rolled split scripts | Replaces ad-hoc shard naming, partial-write cleanup, and missing manifests with one documented command | A recorded 4.1 GiB gzip split completed in `10:02.18`, produced `125` shards, and stayed around `41.3 MiB` peak RSS | Your current script already gives deterministic shards, manifesting, and failure-safe writes |

## What `dimlox` is better at

- staying in one mental model across Azure, GCS, and local filesystems
- giving you `doctor`, `ls`, `get`, `inspect`, and `split` as one operator workflow
- handling large text files with stream, range, and binary split modes in one CLI
- making outputs reproducible through manifests and `.part` -> rename safety

## What `dimlox` does not claim

- universal performance wins over every provider-native tool
- server-side same-provider copies in `v0.1.0`
- zero setup; you still need the provider auth that your environment expects

## Recommended way to evaluate it

1. Run `dimlox doctor` against the provider you already use.
2. Download one real file with `dimlox get`.
3. Run `inspect --detect` and `inspect --wc` on that file.
4. Split it once with `--dry-run`, then once with `--manifest`.

If that replaces two or three shell steps you already repeat, `dimlox` is doing
its job.

## Evidence trail

- `.plans/dogfooding/phase2/README.md`
- `.plans/dogfooding/phase4/README.md`

## Related docs

- [`docs/adoption/positioning.md`](positioning.md)
- [`docs/adoption/quickstart.md`](quickstart.md)
- [`docs/adoption/recipes.md`](recipes.md)
