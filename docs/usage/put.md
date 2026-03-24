# put

`dimlox put` uploads a local file to cloud or local storage with streaming IO and
optional gzip conversion before upload.

## Quick start

```bash
dimlox put "/tmp/orders.psv.gz" "gs://example-bucket/data/orders.psv.gz"
```

## Usage

```bash
dimlox put <src-path> <dst-uri>
```

## Flags

### Command flags

| Flag | Default | What it does | Example |
|---|---|---|---|
| `--block-mb` | `32` | Set multipart upload chunk size in MiB | `dimlox put --block-mb 64 "/tmp/orders.psv.gz" "gs://example-bucket/data/orders.psv.gz"` |
| `--concurrency` | `4` | Set the number of upload workers | `dimlox put --concurrency 8 "/tmp/orders.psv.gz" "gs://example-bucket/data/orders.psv.gz"` |
| `--content-type` | `""` | Override the destination content type | `dimlox put --content-type text/plain "/tmp/orders.psv" "azblob://exampleaccount/example-container/data/orders.psv"` |
| `--compress` | `false` | Gzip-compress uncompressed text before upload | `dimlox put --compress "/tmp/orders.psv" "gs://example-bucket/data/orders.psv.gz"` |

### Common flags

| Flag | Default | What it does | Example |
|---|---|---|---|
| `--az-profile` | `""` | Select the Azure CLI profile for Azure uploads | `dimlox put --az-profile client-a "/tmp/orders.psv" "azblob://exampleaccount/example-container/data/orders.psv"` |
| `--gcp-project` | `GCLOUD_PROJECT` / `GOOGLE_CLOUD_PROJECT` / `""` | Provide a requester-pays project for GCS uploads | `dimlox put --gcp-project example-project "/tmp/orders.psv" "gs://example-bucket/data/orders.psv"` |
| `--landing` | `DIMLOX_LANDING_DIR` / `""` | Choose where temporary gzip staging files are created | `dimlox put --compress --landing "/tmp/dimlox" "/tmp/orders.psv" "gs://example-bucket/data/orders.psv.gz"` |
| `--log-level` | effective `info` | Set CLI log verbosity | `dimlox put --log-level debug "/tmp/orders.psv" "gs://example-bucket/data/orders.psv"` |

## Behavior notes

- `--compress` creates a temporary gzip file in `--landing`, `DIMLOX_LANDING_DIR`, or the current working directory, uploads it, then removes it
- if `--content-type` is omitted, `dimlox` infers content type from the source filename
- progress goes to `stderr` with the same TTY/JSON behavior as `get`

## Examples

### Upload a prepared gzip file

```bash
dimlox put "/tmp/orders.psv.gz" "azblob://exampleaccount/example-container/data/orders.psv.gz"
```

### Upload plain text and compress it first

```bash
dimlox put --compress --landing "/tmp/dimlox" \
  "/tmp/orders.psv" \
  "gs://example-bucket/data/orders.psv.gz"
```

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Upload succeeded |
| `1` | Generic runtime failure |
| `33` | Disk became full while creating a staged upload artifact |
| `40` | Unsupported or invalid URI / arguments |
| `70` | Authentication failed for the destination provider |

## Related docs

- [`docs/usage/get.md`](get.md)
- [`docs/usage/cp.md`](cp.md)
