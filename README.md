# dimlox

Data in motion — large-file transfer, inspection, and splitting across Azure Blob Storage, Google Cloud Storage, and local filesystems.

dimlox is designed for files too large to load into memory. All operations stream — peak RSS is a first-class correctness criterion.

## Commands

| Command | Purpose |
|---|---|
| `doctor` | Auth and connectivity checks |
| `ls` | Container/bucket listing |
| `get` | Download (parallel, streaming) |
| `put` | Upload (resumable) |
| `cp` | Cross-provider copy |
| `inspect` | Streaming wc, head/mid/tail, encoding detection |
| `split` | Row-split, range-split, binary-split with manifest |

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

- Go 1.23+
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

## Install

```bash
make install        # copies binary to ~/.local/bin/dimlox
```

Default install location:

- Linux / macOS: `~/.local/bin/dimlox`
- Windows: `%APPDATA%/dimlox/bin/dimlox.exe`

## Authentication

**Azure Blob Storage** — uses `DefaultAzureCredential`. Activate the right CLI profile first:

```bash
az-profile client-a   # or whatever alias sets AZURE_CONFIG_DIR
```

Setup guide: `docs/setup/azure-cli.md`

**Google Cloud Storage** — uses Application Default Credentials:

```bash
gcloud auth application-default login
```

Setup guide: `docs/setup/gcloud-storage.md`

## Version

```bash
dimlox --version
dimlox version
```

Version is embedded at build time via `-ldflags`. The `VERSION` file at the repo root is the single source of truth (format: `vX.Y.Z`).

```bash
make version-patch   # v0.1.0 → v0.1.1
make version-minor   # v0.1.0 → v0.2.0
make version-major   # v0.1.0 → v1.0.0
make version-set V=v0.2.0
```

## Development

See [`AGENTS.md`](AGENTS.md) for the full contributor guide — roles, phase gates, PR workflow, commit attribution, and quality gate requirements.
