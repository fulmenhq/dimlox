# get

`dimlox get` downloads a cloud or local object to a local file with streaming IO,
parallel chunking where supported, and optional checksum verification.

## Quick start

```bash
dimlox get --verify "gs://example-bucket/data/orders.psv.gz" "/tmp/orders.psv.gz"
```

## Usage

```bash
dimlox get <src-uri> [dst-path]
```

If `dst-path` is omitted, `dimlox` chooses a destination under `--landing`,
`DIMLOX_LANDING_DIR`, or the current working directory.

## Flags

### Command flags

| Flag | Default | What it does | Example |
|---|---|---|---|
| `--block-mb` | `32` | Set the transfer chunk size in MiB | `dimlox get --block-mb 64 "gs://example-bucket/data/orders.psv.gz" "/tmp/orders.psv.gz"` |
| `--concurrency` | `8` | Set the number of parallel download workers | `dimlox get --concurrency 16 "gs://example-bucket/data/orders.psv.gz" "/tmp/orders.psv.gz"` |
| `--compress` | `false` | Gzip-compress uncompressed text while writing locally | `dimlox get --compress "gs://example-bucket/data/orders.psv" "/tmp/orders.psv.gz"` |
| `--overwrite` | `false` | Replace an existing destination file | `dimlox get --overwrite "gs://example-bucket/data/orders.psv.gz" "/tmp/orders.psv.gz"` |
| `--verify` | `false` | Verify checksum metadata when available | `dimlox get --verify "gs://example-bucket/data/orders.psv.gz" "/tmp/orders.psv.gz"` |

### Common flags

| Flag | Default | What it does | Example |
|---|---|---|---|
| `--az-profile` | `""` | Select the Azure CLI profile for Azure downloads | `dimlox get --az-profile client-a "azblob://exampleaccount/example-container/data/orders.psv.gz" "/tmp/orders.psv.gz"` |
| `--gcp-project` | `GCLOUD_PROJECT` / `GOOGLE_CLOUD_PROJECT` / `""` | Provide a requester-pays project for GCS downloads | `dimlox get --gcp-project example-project "gs://example-bucket/data/orders.psv.gz" "/tmp/orders.psv.gz"` |
| `--landing` | `DIMLOX_LANDING_DIR` / `""` | Set the default destination base when `dst-path` is omitted | `dimlox get --landing "/tmp/dimlox" "gs://example-bucket/data/orders.psv.gz"` |
| `--log-level` | effective `info` | Set CLI log verbosity | `dimlox get --log-level debug "gs://example-bucket/data/orders.psv.gz" "/tmp/orders.psv.gz"` |

## Behavior notes

- writes through `<destination>.part` and renames on success
- when `--compress` is used, checksum verification is skipped for the transformed output
- progress goes to `stderr`
- interactive terminals get a live progress line; piped runs get JSON Lines events

## Examples

### Download to an explicit path

```bash
dimlox get "azblob://exampleaccount/example-container/data/orders.psv.gz" "/tmp/orders.psv.gz"
```

### Download a large cloud file with verification

Use this when you want the simplest safe path from a cloud object to a local
working file:

```bash
dimlox get --verify \
  "gs://example-bucket/test/fileio/price_20240824.psv.gz" \
  "$HOME/work/dimlox/price_20240824.psv.gz"
```

What to expect:

- the file downloads to `price_20240824.psv.gz.part` first, then renames on success
- `--verify` checks provider checksum metadata when available
- progress writes to `stderr`
- interactive terminals show a live progress line
- piped or captured runs emit JSON Lines progress events instead

This is a good first step before `inspect` or `split` when you want the cloud
transfer to complete and verify before any downstream processing starts.

### Download into the landing area

```bash
dimlox get --landing "/tmp/dimlox" "gs://example-bucket/data/orders.psv.gz"
```

### Convert plain text to gzip on the way down

```bash
dimlox get --compress "gs://example-bucket/data/orders.psv" "/tmp/orders.psv.gz"
```

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Download succeeded |
| `1` | Generic runtime failure |
| `33` | Disk became full while writing |
| `40` | Unsupported or invalid URI / arguments |
| `63` | Checksum mismatch when `--verify` is enabled |
| `70` | Authentication failed for the source provider |

## Related docs

- [`docs/usage/put.md`](put.md)
- [`docs/usage/cp.md`](cp.md)
- [`docs/usage/split.md`](split.md)
