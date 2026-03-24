# ls

`dimlox ls` lists local files, cloud prefixes, buckets, and containers without
buffering the full result set into memory.

## Quick start

```bash
dimlox ls --long --limit 10 "gs://example-bucket/data/"
```

## Usage

```bash
dimlox ls <uri>
```

## Flags

### Command flags

| Flag | Default | What it does | Example |
|---|---|---|---|
| `--long`, `-l` | `false` | Show size, timestamps, content type, and ETag columns | `dimlox ls --long "azblob://exampleaccount/example-container/data/"` |
| `--hash` | `false` | Include MD5 or CRC32C metadata when available | `dimlox ls --hash "gs://example-bucket/data/"` |
| `--recursive` | `false` | Walk all objects below the prefix instead of one level | `dimlox ls --recursive "/tmp/data"` |
| `--limit` | `0` | Stop after `N` results; `0` means unlimited | `dimlox ls --limit 25 "gs://example-bucket/data/"` |
| `--format` | `text` | Choose `text` or `json` output | `dimlox ls --format json "azblob://exampleaccount/example-container/data/"` |

### Common flags

| Flag | Default | What it does | Example |
|---|---|---|---|
| `--az-profile` | `""` | Select the Azure CLI profile for Azure listings | `dimlox ls --az-profile client-a "azblob://exampleaccount/example-container/"` |
| `--gcp-project` | `GCLOUD_PROJECT` / `GOOGLE_CLOUD_PROJECT` / `""` | Provide a requester-pays project for GCS listings | `dimlox ls --gcp-project example-project "gs://example-bucket/data/"` |
| `--log-level` | effective `info` | Set CLI log verbosity | `dimlox ls --log-level debug "gs://example-bucket/data/"` |

## Output

- `text` output prints names only unless `--long` or `--hash` is set
- prefix entries end with `/`
- `json` output is JSON Lines, one object per line, not a single array

## Examples

### List the top of a bucket or container

```bash
dimlox ls "gs://example-bucket/"
dimlox ls "azblob://exampleaccount/example-container/"
```

### List recursively with metadata

```bash
dimlox ls --recursive --long --hash "/tmp/data"
```

### Emit JSON Lines for another tool

```bash
dimlox ls --format json --limit 5 "gs://example-bucket/data/"
```

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Listing succeeded |
| `1` | Provider or filesystem listing failed |
| `40` | Invalid format or unsupported URI |
| `70` | Authentication failed for the target provider |

## Related docs

- [`docs/usage/doctor.md`](doctor.md)
- [`docs/usage/get.md`](get.md)
