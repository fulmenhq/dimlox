# dimlox

[![CI](https://github.com/fulmenhq/dimlox/actions/workflows/ci.yml/badge.svg)](https://github.com/fulmenhq/dimlox/actions/workflows/ci.yml)

Moving and shaping structured data across the clouds.

`dimlox` moves, inspects, and splits large files across Azure Blob Storage,
Google Cloud Storage, and local filesystems without loading whole files into
memory.

It is built for technical users who move large datasets for a living: data
engineers and devops operators who need reliable transfers, plus QA and
pipeline testers who need repeatable inspection and sharding.

It is built for the workflows that usually turn into brittle shell pipelines or
custom one-off scripts:

- download a multi-GB file without guessing whether it will fit in RAM
- inspect row counts, samples, and delimiters before touching the full payload
- split large text files into repeatable shards with manifests and atomic writes

## Start here

- New user path: [`docs/adoption/quickstart.md`](docs/adoption/quickstart.md)
- Why this exists: [`docs/adoption/positioning.md`](docs/adoption/positioning.md)
- Honest comparisons: [`docs/adoption/vs-existing.md`](docs/adoption/vs-existing.md)
- Workflow recipes: [`docs/adoption/recipes.md`](docs/adoption/recipes.md)

## Install quick start

### Homebrew

```bash
brew install fulmenhq/tap/dimlox
```

### Scoop

```bash
scoop bucket add fulmenhq https://github.com/fulmenhq/scoop-bucket
scoop install fulmenhq/dimlox
```

### Direct release download

If you prefer manual installs, download the platform binary plus checksum files
from the GitHub Releases page:

- `https://github.com/fulmenhq/dimlox/releases`

## Quick start

### Download a file

```bash
dimlox get --verify "gs://example-bucket/data/orders.psv.gz" "/tmp/orders.psv.gz"
```

### Inspect shape before processing

```bash
dimlox inspect --detect "/tmp/orders.psv.gz"
dimlox inspect --wc "/tmp/orders.psv.gz"
```

### Split a large file into row-based shards

```bash
dimlox split --rows 5000000 --header --manifest --out-dir "/tmp/shards" "/tmp/orders.psv.gz"
```

## Command guide

| Command | Purpose | Usage guide |
|---|---|---|
| `doctor` | Check auth, connectivity, and target metadata probes | [`docs/usage/doctor.md`](docs/usage/doctor.md) |
| `ls` | List local files, buckets, containers, and prefixes | [`docs/usage/ls.md`](docs/usage/ls.md) |
| `get` | Download to a local path with streaming progress | [`docs/usage/get.md`](docs/usage/get.md) |
| `put` | Upload a local file to cloud or local storage | [`docs/usage/put.md`](docs/usage/put.md) |
| `cp` | Copy between providers through a landing area | [`docs/usage/cp.md`](docs/usage/cp.md) |
| `inspect` | Stream counts, samples, and delimiter/encoding detection | [`docs/usage/inspect.md`](docs/usage/inspect.md) |
| `split` | Create stream, range, or binary shards with manifests | [`docs/usage/split.md`](docs/usage/split.md) |

## Common workflows

### Validate auth before a run

```bash
dimlox doctor
dimlox doctor "azblob://exampleaccount/example-container/data/orders.psv"
```

### List a prefix before copying or splitting

```bash
dimlox ls --long --limit 20 "gs://example-bucket/data/"
```

### Copy between providers through a local landing file

```bash
dimlox cp --verify --landing "/tmp/dimlox" \
  "gs://example-bucket/data/orders.psv" \
  "azblob://exampleaccount/example-container/data/orders.psv"
```

### Dry-run a split before writing shards

```bash
dimlox split --dry-run --rows 5000000 --header --out-dir "/tmp/shards" \
  "gs://example-bucket/data/orders.psv"
```

### Inspect compressed cloud data safely

```bash
dimlox inspect --head 5 "gs://example-bucket/data/orders.psv.gz"
dimlox inspect --tail 5 --force-stream "gs://example-bucket/data/orders.psv.gz"
```

`inspect --mid` and `inspect --tail` refuse compressed cloud sources by default
because those operations require a full forward stream over the network. The
usage guide shows the download-first workflow when you want the safer option.

## Supported URI forms

```
azblob://<account>/<container>/<path>
https://<account>.blob.core.windows.net/<container>/<path>
gs://<bucket>/<path>
https://storage.googleapis.com/<bucket>/<path>
/absolute/local/path
relative/local/path
```

## Prerequisites

- Go 1.25+
- Azure CLI (`az`) with an active login for AZS operations
- `gcloud` with Application Default Credentials for GCS operations

## Bootstrap (fresh clone)

```bash
make bootstrap
```

This runs the trust-anchor chain: `curl` → `sfetch` → `goneat` → dev tools.
On a machine where all tools are already installed it is a no-op.

## Build

```bash
make build          # current platform → bin/dimlox_<os>_<arch>
make build-all      # all platforms → dist/release/
```

## Quality gate

Run before every commit (required by AGENTS.md):

```bash
make check          # go fmt + go vet + go test
```

Full assess (format, lint, security — requires goneat):

```bash
make assess
```

CI runs `make fmt`, `go vet ./...`, `make lint`, `make test-short`, and
`make build` on pull requests and pushes to `main`. Separate jobs cover Linux
race testing with `CGO_ENABLED=1`, cross-builds, and native smoke checks on
Linux and Windows arm64 runners.

## Install

Package manager installs:

```bash
brew install fulmenhq/tap/dimlox
```

```bash
scoop bucket add fulmenhq https://github.com/fulmenhq/scoop-bucket
scoop install fulmenhq/dimlox
```

Local developer install:

```bash
make install        # copies binary to ~/.local/bin/dimlox
```

Default install location:

- Linux / macOS: `~/.local/bin/dimlox`
- Windows: `%LOCALAPPDATA%/Programs/dimlox/bin/dimlox.exe`

For Windows developer shells, run `make install-path` once to add the install
directory to the user PATH. Long-term end-user distribution is expected to go
through Scoop rather than `make install`.

## Windows status

Windows is a supported target for local workflows and release builds.

- cross-compiles are expected to pass for Windows amd64 and arm64
- local file I/O uses OS-native paths, but manifest shard paths remain portable forward slashes
- `split` preflights planned output paths at the 260-character portability limit before writing files

Known limitations:

- native smoke now runs in CI, but maintainer validation on a real Windows workstation is still useful before release
- legacy Windows consoles may fall back to plain progress output; non-TTY runs still emit structured JSON Lines progress on `stderr`

## Authentication

**Azure Blob Storage** — uses `DefaultAzureCredential`. If you use named Azure
CLI profiles, select the right `AZURE_CONFIG_DIR` in your shell and pass
`--az-profile <name>` to `dimlox`.

Setup guide: `docs/setup/azure-cli.md`

**Google Cloud Storage** — uses Application Default Credentials:

```bash
gcloud auth application-default login
```

Setup guide: `docs/setup/gcloud-storage.md`

If you only need command examples and behavior, start with the usage docs above.
If you need local credential setup, read the setup guides first.

## Output and safety notes

- Transfer progress writes to `stderr`
- Interactive terminals get a live progress line; piped runs get JSON Lines progress
- `get` and `split` write through `.part` files and rename on success
- `split --manifest` writes JSON Lines metadata for each shard
- `split --dry-run` prints the shard plan and writes nothing

## Version

```bash
dimlox --version
dimlox version
```

Version is embedded at build time via `-ldflags`. The `VERSION` file at the repo root is the single source of truth (format: `vX.Y.Z`).

```bash
make version-patch
make version-minor
make version-major
make version-set V=vX.Y.Z
```

Release automation is split between GitHub Actions and local signing:

- CI builds draft releases for `v*` tags
- checksum signing and provenance upload stay local
- use `DIMLOX_RELEASE_TAG=vX.Y.Z` for local release helper targets

Maintainer release steps are documented in `RELEASE_CHECKLIST.md`.

## Development

See [`AGENTS.md`](AGENTS.md) for the full contributor guide — roles, phase gates, PR workflow, commit attribution, and quality gate requirements.
