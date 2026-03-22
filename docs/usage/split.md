# split

`dimlox split` breaks large files into reproducible shard files without loading
the whole source into memory. It supports stream-aware text splitting, range
splitting for uncompressed cloud text, and exact-byte binary splitting.

## Quick start

```bash
dimlox split --rows 5000000 --header --manifest --out-dir "/tmp/shards" "/tmp/orders.psv.gz"
```

## Usage

```bash
dimlox split <uri>
```

## Flags

### Command flags

| Flag | Default | What it does | Example |
|---|---|---|---|
| `--mode` | `auto` | Choose `auto`, `stream`, `range`, or `binary` splitting | `dimlox split --mode range --rows 1000000 "gs://example-bucket/data/orders.psv"` |
| `--rows` | `0` | Set the maximum data rows per shard for text modes | `dimlox split --rows 5000000 "/tmp/orders.psv.gz"` |
| `--bytes` | `500` | Set the maximum shard size in MiB | `dimlox split --bytes 256 --mode binary "/tmp/archive.bin"` |
| `--out-dir` | `""` | Choose where shard files are written | `dimlox split --out-dir "/tmp/shards" "/tmp/orders.psv.gz"` |
| `--out-fmt` | `match` | Choose `match`, `text`, or `gz` output format | `dimlox split --out-fmt gz "/tmp/orders.psv"` |
| `--header` | `false` | Copy the first line into every text shard | `dimlox split --rows 5000000 --header "/tmp/orders.psv"` |
| `--delimiter` | `""` | Override text delimiter detection | `dimlox split --delimiter '|' "/tmp/orders.psv"` |
| `--encoding` | `""` | Override text encoding detection | `dimlox split --encoding UTF-8 "/tmp/orders.psv"` |
| `--manifest` | `true` | Write a JSON Lines manifest alongside shards | `dimlox split --manifest=false "/tmp/orders.psv"` |
| `--dry-run` | `false` | Print the planned shard layout and write nothing | `dimlox split --dry-run --rows 5000000 "/tmp/orders.psv"` |
| `--sample-bytes` | `65536` | Set the bounded sample size for delimiter and encoding detection | `dimlox split --sample-bytes 131072 "/tmp/orders.psv"` |

### Common flags

| Flag | Default | What it does | Example |
|---|---|---|---|
| `--az-profile` | `""` | Select the Azure CLI profile for Azure range or stream splits | `dimlox split --az-profile client-a --rows 5000000 "azblob://exampleaccount/example-container/data/orders.psv"` |
| `--gcp-project` | `GCLOUD_PROJECT` / `GOOGLE_CLOUD_PROJECT` / `""` | Provide a requester-pays project for GCS splits | `dimlox split --gcp-project example-project --rows 5000000 "gs://example-bucket/data/orders.psv"` |
| `--landing` | `DIMLOX_LANDING_DIR` / `""` | Provide the default shard directory when `--out-dir` is omitted | `dimlox split --landing "/tmp/dimlox" --rows 5000000 "/tmp/orders.psv"` |
| `--log-level` | effective `info` | Set CLI log verbosity | `dimlox split --log-level debug --rows 5000000 "/tmp/orders.psv"` |

## Mode selection

### `auto`

`auto` resolves in this order:

1. compressed source -> `stream`
2. uncompressed cloud text source -> `range`
3. other text-like source -> `stream`
4. everything else -> `binary`

Text-like means a `text/*` content type or a filename that looks like `.csv`,
`.psv`, `.tsv`, `.json`, or `.txt` after trimming `.gz`.

### `stream`

Use `stream` for:

- compressed text sources
- local text files
- any text source where a single forward pass is acceptable

How it works:

- opens one forward reader
- detects delimiter/encoding unless you override them
- rotates shards when `--rows` or `--bytes` limits are reached
- can replicate the first line into every shard when `--header` is enabled

Tradeoff:

- on cloud sources, stream mode depends on one long-lived reader, so the auth
  token used for that stream must last for the full split operation

### `range`

Use `range` for:

- uncompressed cloud text files
- long-running cloud splits where bounded chunk reads are safer than one long stream

How it works:

- reads the source through bounded HTTP range requests
- reassembles lines that cross range boundaries
- reads the header through bounded probes until newline or EOF
- can fetch a fresh token per range request through the provider SDK path

Restrictions:

- only for uncompressed cloud text sources
- rejected for local files, compressed sources, and non-text-like data

### `binary`

Use `binary` for:

- archives or other non-text payloads
- exact byte-based chunking where delimiters do not matter

How it works:

- copies raw bytes into shard files
- ignores `--delimiter`, `--encoding`, and `--header`
- writes through a fixed-size copy buffer instead of allocating shard-sized memory

## Output layout

Text shards follow this pattern:

```text
<stem>_shard_NNNN.<ext>
```

Binary shards follow this pattern:

```text
<stem>_part_NNNN.bin
```

Examples:

- `orders_shard_0001.psv`
- `orders_shard_0002.psv.gz`
- `archive_part_0003.bin`

`--out-fmt` controls the output extension and compression behavior:

- `match` keeps the source format when possible
- `text` forces plain-text shard files for text modes
- `gz` forces gzip shard files for text modes

`--out-dir` resolution order:

1. explicit `--out-dir`
2. `--landing`
3. `DIMLOX_LANDING_DIR`
4. current working directory

## Windows readiness notes

- before any real split I/O, `dimlox` preflights the longest planned output path
- at the 260-character portability limit, Windows errors before writing files and other platforms emit a warning
- this check runs in both normal and `--dry-run` mode so you can shorten `--out-dir` before committing to output
- source filename stems that contain Windows-illegal characters such as `< > : " / \ | ? *` are rejected before writing shards
- manifest `shard_file` values stay forward-slash relative paths even when the local filesystem uses backslashes

## Manifest

Manifest output is enabled by default.

- file name: `<stem>_manifest.jsonl`
- format: JSON Lines, one shard per line
- includes source URI, source metadata, shard index, shard path, row count,
  byte count, split mode, delimiter, encoding, header copy flag, and completion time

In `--dry-run`, the result still reports the planned manifest path, but no file is written.

## Dry run

Use `--dry-run` when you want the plan without side effects.

```bash
dimlox split --dry-run --rows 5000000 --header --out-dir "/tmp/shards" "/tmp/orders.psv.gz"
```

Dry run behavior:

- prints source, mode, output directory, and manifest path
- prints one JSON object per planned shard
- exits `0` on success
- writes no shard files and no manifest file

## `.part` safety

Every real shard write goes through a temporary `.part` file before rename.

- complete shard -> `.part` renamed to final name
- interrupted run -> incomplete shard stays as `.part`
- manifest uses the same pattern

This keeps partially written outputs easy to spot and avoids corrupt final shard names.

## Recipes

### Split a large gzip file into 5M-row shards with headers

```bash
dimlox split --rows 5000000 --header --manifest --out-dir "/tmp/shards" "/tmp/orders.psv.gz"
```

### Split a remote uncompressed text file with range mode

```bash
dimlox split --mode range --rows 1000000 --header --out-dir "/tmp/shards" \
  "gs://example-bucket/data/orders.psv"
```

### Dry-run to estimate shard count first

```bash
dimlox split --dry-run --rows 1000000 --header --out-dir "/tmp/shards" \
  "azblob://exampleaccount/example-container/data/orders.psv"
```

### Split binary data by size

```bash
dimlox split --mode binary --bytes 256 --out-dir "/tmp/shards" "/tmp/archive.bin"
```

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Split succeeded or dry-run plan succeeded |
| `1` | Operational failure |
| `2` | Invalid mode, invalid limits, unsupported URI, or other bad input |
| `4` | Disk became full while writing shards or manifest |

## Related docs

- [`docs/usage/inspect.md`](inspect.md)
- [`docs/usage/get.md`](get.md)
