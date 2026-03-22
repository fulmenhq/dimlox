# cp

`dimlox cp` copies data between providers through a local landing file. It is the
safe cross-provider path when you need one reproducible command instead of a
download script plus a separate upload step.

## Quick start

```bash
dimlox cp --landing "/tmp/dimlox" \
  "gs://example-bucket/data/orders.psv" \
  "azblob://exampleaccount/example-container/data/orders.psv"
```

## Usage

```bash
dimlox cp <src-uri> <dst-uri>
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

## Examples

### Copy from GCS to Azure with verification

```bash
dimlox cp --verify --landing "/tmp/dimlox" \
  "gs://example-bucket/data/orders.psv" \
  "azblob://exampleaccount/example-container/data/orders.psv"
```

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
