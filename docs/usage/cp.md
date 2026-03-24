# cp

`dimlox cp` copies data between providers through a local landing file. It now
supports single-file copies, positional multi-source copies, glob expansion, and
JSONL-driven transfer batches.

## Quick start

```bash
dimlox cp --landing "/tmp/dimlox" \
  "gs://example-bucket/data/orders.psv" \
  "azblob://exampleaccount/example-container/data/orders.psv"
```

```bash
dimlox cp --dry-run "gs://example-bucket/data/orders_*.psv" \
  "azblob://exampleaccount/example-container/data/"
```

## Usage

```bash
dimlox cp [flags] <src-uri>... <dst-uri>
dimlox cp [flags] --from-file <path>
```

## Flags

### Command flags

| Flag | Default | What it does | Example |
|---|---|---|---|
| `--block-mb` | `32` | Set chunk size for the download and upload legs | `dimlox cp --block-mb 64 "gs://example-bucket/data/orders.psv" "azblob://exampleaccount/example-container/data/orders.psv"` |
| `--concurrency` | `8` | Set parallel workers for the download leg | `dimlox cp --concurrency 16 "gs://example-bucket/data/orders.psv" "azblob://exampleaccount/example-container/data/orders.psv"` |
| `--compress` | `false` | Gzip-compress uncompressed text in landing before upload | `dimlox cp --compress "gs://example-bucket/data/orders.psv" "azblob://exampleaccount/example-container/data/orders.psv.gz"` |
| `--keep-landing` | `false` | Keep the intermediate landing file after upload | `dimlox cp --keep-landing --landing "/tmp/dimlox" "gs://example-bucket/data/orders.psv" "azblob://exampleaccount/example-container/data/orders.psv"` |
| `--verify` | `false` | Verify checksum metadata on the download leg | `dimlox cp --verify "gs://example-bucket/data/orders.psv" "azblob://exampleaccount/example-container/data/orders.psv"` |
| `--from-file` | `""` | Read JSONL `src` / `dst` pairs from a file | `dimlox cp --from-file transfers.jsonl` |
| `--continue-on-error` | `false` | Attempt all planned transfers and report failures at the end | `dimlox cp --continue-on-error --from-file transfers.jsonl` |
| `--dry-run` | `false` | Print the resolved transfer plan without copying | `dimlox cp --dry-run "gs://example-bucket/data/orders_*.psv" "azblob://exampleaccount/example-container/data/"` |
| `--max-sources` | `1000` | Fail preflight when glob expansion resolves more than N files | `dimlox cp --max-sources 2500 "gs://example-bucket/data/orders_*.psv" "azblob://exampleaccount/example-container/data/"` |
| `--parallel` | `1` | Reserved for a future concurrent batch mode; values above `1` are rejected today | `dimlox cp --parallel 1 --from-file transfers.jsonl` |

### Common flags

| Flag | Default | What it does | Example |
|---|---|---|---|
| `--az-profile` | `""` | Select the Azure CLI profile for Azure endpoints in the copy | `dimlox cp --az-profile client-a "gs://example-bucket/data/orders.psv" "azblob://exampleaccount/example-container/data/orders.psv"` |
| `--gcp-project` | `GCLOUD_PROJECT` / `GOOGLE_CLOUD_PROJECT` / `""` | Provide a requester-pays project for GCS endpoints in the copy | `dimlox cp --gcp-project example-project "gs://example-bucket/data/orders.psv" "azblob://exampleaccount/example-container/data/orders.psv"` |
| `--landing` | `DIMLOX_LANDING_DIR` / `""` | Choose where the intermediate file is written | `dimlox cp --landing "/tmp/dimlox" "gs://example-bucket/data/orders.psv" "azblob://exampleaccount/example-container/data/orders.psv"` |
| `--log-level` | effective `info` | Set CLI log verbosity | `dimlox cp --log-level debug "gs://example-bucket/data/orders.psv" "azblob://exampleaccount/example-container/data/orders.psv"` |

## Behavior notes

- `cp` is always a two-leg flow: download to landing, then upload from landing
- `--verify` checks the download leg only
- the landing file is removed unless `--keep-landing` is set
- progress is reported separately for the `get` and `put` legs on `stderr`
- all multi-file modes use plan -> validate -> execute; bad URIs, collisions, and invalid destinations fail before transfers start
- multi-file destinations must end with `/`; trailing `/` is the universal prefix signal for cloud and local targets
- basename collisions are a hard preflight error; `a/orders.psv` and `b/orders.psv` cannot both map to `dst/orders.psv`
- glob matching uses provider listing plus client-side `path.Match` filtering over provider-relative object keys

## Examples

### Copy from GCS to Azure with verification

```bash
dimlox cp --verify --landing "/tmp/dimlox" \
  "gs://example-bucket/data/orders.psv" \
  "azblob://exampleaccount/example-container/data/orders.psv"
```

### Copy between cloud providers through the landing area

Use this when you need a simple cross-provider transfer and want `dimlox` to
handle the local staging step for you:

```bash
dimlox cp --verify --landing "$HOME/work/dimlox" \
  "azblob://exampleaccount/example-container/data/product_20230923.psv" \
  "gs://example-bucket/data/product_20230923.psv"
```

What to expect:

- `cp` downloads the source object into the landing directory first
- `cp` then uploads that staged file to the destination provider
- the landing file is removed automatically on success unless `--keep-landing` is set
- progress is reported separately for the `get` and `put` legs
- `--verify` checks the download leg before the upload starts

This is the normal cross-cloud path today. It is intentionally a local
staging flow, not a direct provider-to-provider stream.

### Copy multiple explicit sources into one destination prefix

```bash
dimlox cp \
  "gs://example-bucket/data/orders_2024.psv" \
  "gs://example-bucket/data/orders_2025.psv" \
  "azblob://exampleaccount/example-container/archive/"
```

What to expect:

- the trailing `/` on the destination tells `dimlox` to map each source by basename
- each file is transferred sequentially through the normal landing flow
- `cp` prints a completion summary after the batch finishes

### Expand a glob on the source provider

```bash
dimlox cp --dry-run "gs://example-bucket/data/orders_*.psv" \
  "azblob://exampleaccount/example-container/archive/"
```

What to expect:

- quote the source so your shell does not expand it locally
- `dimlox` lists a bounded prefix on the source provider, then filters matches client-side
- `--dry-run` prints the full source -> destination plan before you move data

### Run a JSONL batch file

```bash
dimlox cp --continue-on-error --from-file transfers.jsonl
```

Example JSONL input:

```json
{"src":"gs://example-bucket/data/orders_2024.psv","dst":"azblob://exampleaccount/example-container/archive/orders_2024.psv"}
{"src":"azblob://exampleaccount/example-container/data/orders_2025.psv","dst":"/tmp/orders_2025.psv"}
```

Notes:

- comments starting with `#` or `//` are ignored
- the file is fully validated before any transfers begin
- duplicate destination paths fail preflight

### Keep the landing file for a later retry or inspection

```bash
dimlox cp --keep-landing --landing "/tmp/dimlox" \
  "azblob://exampleaccount/example-container/data/orders.psv.gz" \
  "/tmp/archive/orders.psv.gz"
```

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Copy succeeded |
| `1` | Operational failure |
| `2` | Unsupported or invalid URI |
| `3` | Checksum mismatch when `--verify` is enabled |
| `4` | Disk became full while staging the landing file |

## Related docs

- [`docs/usage/get.md`](get.md)
- [`docs/usage/put.md`](put.md)
