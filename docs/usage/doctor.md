# doctor

`dimlox doctor` tells you whether `dimlox` can authenticate, reach the target
provider, and read basic metadata before you start a larger operation.

## Quick start

```bash
dimlox doctor
```

## What it does

- with no target URI, checks local filesystem access plus Azure and GCS auth
- with a target URI, resolves the provider, normalizes the URI, then probes the
  object, prefix, bucket, container, or local file
- reports Azure and GCS token validity windows without printing token values

## Probe scope

`doctor` narrows its probe set when provider intent is explicit:

- `dimlox doctor` checks `local`, `azblob`, and `gcs`
- `dimlox doctor --az-profile ...` checks `local` and `azblob`
- `dimlox doctor --gcp-project ...` checks `local` and `gcs`
- `dimlox doctor <uri>` checks the target provider only

This keeps provider-specific setup flows focused instead of surfacing unrelated
auth failures.

## Usage

```bash
dimlox doctor [uri]
```

## Flags

### Common flags

| Flag | Default | What it does | Example |
|---|---|---|---|
| `--az-profile` | `""` | Select the Azure CLI profile to use for Azure auth checks | `dimlox doctor --az-profile client-a` |
| `--gcp-project` | `GCLOUD_PROJECT` / `GOOGLE_CLOUD_PROJECT` / `""` | Provide the billing or requester-pays project for GCS probes | `dimlox doctor --gcp-project example-project gs://example-bucket/data/file.psv` |
| `--log-level` | effective `info` | Set CLI log verbosity | `dimlox doctor --log-level debug` |

## Output

### No target URI

`doctor` prints plain-text status lines for each provider, for example:

```text
dimlox v0.1.0 (...)
go: go1.24.x
platform: linux/amd64
local: ok - local filesystem available
azblob: ok - DefaultAzureCredential token acquired (az-profile=client-a) (valid for 72m)
gcs: ok - ADC via local ADC file (~/.config/gcloud/application_default_credentials.json), quota-project=<none> (valid for 45m)
```

### Targeted probe

With a URI, `doctor` prints:

- `provider`
- `normalized`
- object metadata when available (`name`, `size`, `content-type`, `etag`, `last-modified`)
- `latency`

## Examples

### Check all configured providers

```bash
dimlox doctor
```

### Check one Azure blob with a named profile

```bash
dimlox doctor --az-profile client-a \
  "azblob://exampleaccount/example-container/data/orders.psv.gz"
```

### Check a GCS object with an explicit requester-pays project

```bash
dimlox doctor --gcp-project example-project \
  "gs://example-bucket/data/orders.psv"
```

### Check a local file before splitting it

```bash
dimlox doctor "/tmp/orders.psv.gz"
```

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Probe succeeded |
| `1` | Auth, connectivity, or metadata probe failed |
| `2` | Unsupported or invalid URI |

## Troubleshooting

- Azure says `Please run 'az login'`
  - if you are using `--az-profile`, set `AZURE_CONFIG_DIR` to that profile directory first, then run `az login`
- GCS says ADC is missing
  - run `gcloud auth application-default login` or point `GOOGLE_APPLICATION_CREDENTIALS` at an approved credentials file
- Targeted doctor returns `BlobNotFound` or object-not-found
  - auth is probably working; re-check the object path
- GCS works without `--gcp-project` but fails with it
  - the current identity may not have requester-pays or quota-project permission on that project

## Related docs

- [`docs/setup/azure-cli.md`](../setup/azure-cli.md)
- [`docs/setup/gcloud-storage.md`](../setup/gcloud-storage.md)
- [`docs/usage/ls.md`](ls.md)
