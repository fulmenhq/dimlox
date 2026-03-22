# inspect

`dimlox inspect` streams counts, samples, and format detection from large files
without reading the whole file into memory.

## Quick start

```bash
dimlox inspect --detect "/tmp/orders.psv.gz"
```

## Usage

```bash
dimlox inspect <uri>
```

Use one primary mode flag per invocation: `--wc`, `--head`, `--mid`, `--tail`, or
`--detect`.

## Flags

### Command flags

| Flag | Default | What it does | Example |
|---|---|---|---|
| `--wc` | `false` | Count decompressed lines and report stored byte size | `dimlox inspect --wc "gs://example-bucket/data/orders.psv.gz"` |
| `--head` | `0` | Print the first `N` lines | `dimlox inspect --head 5 "/tmp/orders.psv"` |
| `--mid` | `0` | Print `N` lines near the midpoint | `dimlox inspect --mid 10 "/tmp/orders.psv"` |
| `--tail` | `0` | Print the last `N` lines | `dimlox inspect --tail 5 "/tmp/orders.psv"` |
| `--detect` | `false` | Detect encoding, BOM, line ending, and delimiter | `dimlox inspect --detect "gs://example-bucket/data/orders.psv"` |
| `--force-stream` | `false` | Allow expensive full forward-stream fallback for compressed cloud `--mid` or `--tail` | `dimlox inspect --tail 5 --force-stream "gs://example-bucket/data/orders.psv.gz"` |
| `--sample-bytes` | `65536` | Set the bounded sample size for detection | `dimlox inspect --detect --sample-bytes 131072 "/tmp/orders.psv"` |
| `--format` | `text` | Choose `text` or `json` output | `dimlox inspect --wc --format json "/tmp/orders.psv.gz"` |

### Common flags

| Flag | Default | What it does | Example |
|---|---|---|---|
| `--az-profile` | `""` | Select the Azure CLI profile for Azure inspection | `dimlox inspect --az-profile client-a --wc "azblob://exampleaccount/example-container/data/orders.psv.gz"` |
| `--gcp-project` | `GCLOUD_PROJECT` / `GOOGLE_CLOUD_PROJECT` / `""` | Provide a requester-pays project for GCS inspection | `dimlox inspect --gcp-project example-project --wc "gs://example-bucket/data/orders.psv.gz"` |
| `--log-level` | effective `info` | Set CLI log verbosity | `dimlox inspect --log-level debug --detect "/tmp/orders.psv"` |

## How modes behave

- `--wc` streams the file and counts lines
- `--head` always streams forward from the start
- `--mid` and `--tail` use local seeks or cloud ranges when the source is uncompressed
- gzip-compressed files are decompressed transparently

## Compressed cloud guard

`inspect --mid` and `inspect --tail` refuse compressed cloud sources by default.
That guard exists because those operations need a full forward stream over the
network when random access into the decompressed stream is not possible.

Safer pattern:

```bash
dimlox get "gs://example-bucket/data/orders.psv.gz" "/tmp/orders.psv.gz"
dimlox inspect --tail 5 "/tmp/orders.psv.gz"
```

Override only when you understand the cost:

```bash
dimlox inspect --tail 5 --force-stream "gs://example-bucket/data/orders.psv.gz"
```

## Examples

### Count rows in a large gzip file

```bash
dimlox inspect --wc "gs://example-bucket/data/orders.psv.gz"
```

### Sample the top of a local file

```bash
dimlox inspect --head 10 "/tmp/orders.psv"
```

### Detect delimiter and encoding before splitting

```bash
dimlox inspect --detect --sample-bytes 131072 "/tmp/orders.psv"
```

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Inspection succeeded |
| `1` | Operational failure |
| `2` | Invalid format, missing mode, or unsupported URI |

## Related docs

- [`docs/usage/get.md`](get.md)
- [`docs/usage/split.md`](split.md)
