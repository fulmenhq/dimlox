# Quickstart

This is the fastest path from fresh clone to a real `get -> inspect -> split`
workflow.

Assumptions:

- Go is already installed
- `az` and/or `gcloud` are already installed if you plan to hit cloud storage
- you have access to a safe test bucket, container, or local file path

## 1. Build and install

```bash
make bootstrap
make install
dimlox --version
```

If you do not want to install yet, use `go run ./cmd/dimlox` in the commands below.

## 2. Confirm auth and environment

```bash
dimlox doctor
```

If Azure or GCS is not ready yet, finish the setup guide first:

- [`docs/setup/azure-cli.md`](../setup/azure-cli.md)
- [`docs/setup/gcloud-storage.md`](../setup/gcloud-storage.md)

## 3. Download one file

```bash
dimlox get --verify "gs://example-bucket/data/orders.psv.gz" "/tmp/orders.psv.gz"
```

What to expect:

- progress goes to `stderr`
- terminals get a live progress line
- piped runs get JSON Lines progress events instead of ANSI updates
- the file writes through `orders.psv.gz.part` and renames on success

## 4. Inspect before you process

```bash
dimlox inspect --detect "/tmp/orders.psv.gz"
dimlox inspect --wc "/tmp/orders.psv.gz"
```

Use `--detect` to confirm delimiter and encoding before you split. Use `--wc`
when you need a streaming row count.

## 5. Split into repeatable shards

```bash
mkdir -p "/tmp/shards"
dimlox split --rows 5000000 --header --manifest --out-dir "/tmp/shards" "/tmp/orders.psv.gz"
```

What you get:

- shard files under `/tmp/shards`
- a manifest file such as `orders_manifest.jsonl`
- `.part` protection during writes

## 6. Dry-run before a bigger split

```bash
dimlox split --dry-run --rows 5000000 --header --out-dir "/tmp/shards" "/tmp/orders.psv.gz"
```

Use this when you want to confirm mode selection and shard count before writing
files.

## Where to go next

- detailed command flags: [`docs/usage/`](../usage)
- workflow guidance: [`docs/adoption/recipes.md`](recipes.md)
- comparison notes: [`docs/adoption/vs-existing.md`](vs-existing.md)
