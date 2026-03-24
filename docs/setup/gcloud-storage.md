# Google Cloud Storage Setup

`dimlox` uses the Go Cloud Storage client plus Application Default Credentials (ADC).
You do not need GKE, BigQuery, or other Google Cloud tooling for local `dimlox`
development. The important part is ADC for Cloud Storage access.

Preferred team setup is a service-based identity for repeatable access. A
user-based ADC flow also works for local development when the user account has
the required Storage permissions.

## What you need

- `gcloud` installed
- Application Default Credentials set up
- optional: a default project if you use requester-pays buckets

`dimlox` does not require `gsutil`. It also does not shell out to `gcloud storage`
for data operations; it talks to GCS through the Go SDK.

## Recommended auth model

Preferred order for `dimlox` development and testing:

1. service-based identity (preferred)
2. user ADC via `gcloud auth application-default login` (acceptable for local dev)

Why service-based is preferred:

- closer to production-style access control
- easier to share a stable setup across developers and CI
- better fit for billing-sensitive or requester-pays environments
- avoids surprises when a personal user account lacks quota-project permissions

In practice, that usually means one of these patterns:

- service account impersonation
- workload identity / federation
- a dedicated service account with the required Storage permissions

User ADC is still useful for local smoke testing and can work well when:

- the bucket is not requester-pays, and
- the user account already has the necessary access

## Install Google Cloud CLI

### Debian / Ubuntu

```bash
curl https://sdk.cloud.google.com | bash
exec -l "$SHELL"
gcloud version
```

### macOS (Homebrew)

```bash
brew install --cask google-cloud-sdk
gcloud version
```

### Windows

Download and run the interactive installer from
[Google's Cloud SDK install page](https://cloud.google.com/sdk/docs/install#windows).
The installer adds `gcloud` to PATH and offers to run `gcloud init` at the end.

Alternatives:

```powershell
scoop bucket add extras
scoop install gcloud
```

Or via Chocolatey:

```powershell
choco install gcloudsdk
```

After install, close and reopen your terminal, then verify:

```powershell
gcloud version
```

The rest of this guide (ADC login, project config, service-based setup) works the
same on Windows. Environment variables use PowerShell syntax:

```powershell
$env:GOOGLE_APPLICATION_CREDENTIALS = "C:\path\to\service-account.json"
$env:GCLOUD_PROJECT = "example-project"
dimlox doctor --gcp-project example-project
```

## User ADC setup

If you are using your own Google user account for local development, the main step is:

```bash
gcloud auth application-default login
```

If you use multiple browser profiles or want to avoid automatic browser popups,
this variant is often easier to manage:

```bash
gcloud auth application-default login --no-launch-browser
```

That creates the standard ADC file used by `dimlox`.

Optional but useful:

```bash
gcloud auth login
gcloud config set project <project-id>
gcloud config get-value project
```

It is normal for `gcloud auth application-default login` to succeed as a user
login even when `gcloud auth application-default set-quota-project` cannot be
applied. In that case:

- user-based ADC may still work for normal bucket/object access
- requester-pays or quota-project-sensitive access may still fail
- `gcloud config set project <project-id>` is still useful for local CLI context
- `dimlox` can still take `--gcp-project <project-id>` explicitly when needed

## Service-based setup

Exact steps depend on your organization's preferred Google Cloud auth model, but
the goal is the same: make ADC resolve to a service identity instead of a human
user login.

If your team uses service account impersonation or federation, follow that local
setup and then verify ADC before using `dimlox`.

The important requirement for `dimlox` is that the resulting ADC context can:

- list objects in the target bucket
- read object metadata
- use any required billing/quota project for requester-pays access

In many teams, the most practical local option is to point
`GOOGLE_APPLICATION_CREDENTIALS` at a service-account or external-account JSON
that is approved for development use:

```bash
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/approved-dev-credentials.json
dimlox doctor
```

If your org uses impersonation or federation instead, use that flow to produce a
working ADC context first, then verify with `dimlox doctor`.

## Multiple profiles and configurations

This is the main point that trips people up: `gcloud` configurations and ADC are
related, but they are not the same thing.

- `gcloud config configurations ...` controls CLI settings like active account and project
- `gcloud auth application-default login` writes the ADC file used by client libraries like `dimlox`
- switching the active `gcloud` configuration does not automatically switch ADC to a different identity

That means you can end up in mixed states such as:

- CLI account A, but ADC from user B
- active project set in `gcloud`, but ADC still using an older quota-project context
- service-account JSON in `GOOGLE_APPLICATION_CREDENTIALS`, while `gcloud` itself is logged in as a human user

For developers who need more than one mode, the safest patterns are:

### Pattern 1: user ADC for interactive work

Use your own user login when you just need local read/list testing and the bucket
permissions are already granted to your Google account.

```bash
gcloud auth application-default login --no-launch-browser
gcloud config set project <project-id>
dimlox doctor
```

### Pattern 2: service-based credentials for repeatable validation

Use a dedicated service identity when you need stable team behavior, requester-pays
support, or an access model closer to production.

```bash
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-or-external-account.json
export GCLOUD_PROJECT=<project-id>
dimlox doctor --gcp-project <project-id>
```

This works well for `dimlox` because the Google client libraries read
`GOOGLE_APPLICATION_CREDENTIALS` directly. `dimlox` now also supports
`--gcp-profile` plus per-leg `cp` credential flags when one command needs to
touch different GCS identities.

### Pattern 3: separate shell wrappers or env files

If you switch often, keep the setups explicit instead of relying on memory.

Example idea:

```bash
# user mode
unset GOOGLE_APPLICATION_CREDENTIALS
gcloud config set project <project-id>
gcloud auth application-default login --no-launch-browser

# service mode
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-or-external-account.json
export GCLOUD_PROJECT=<project-id>
```

The important habit is to make the active auth mode obvious before running
`dimlox` commands.

One practical rule:

- if `GOOGLE_APPLICATION_CREDENTIALS` is set, `dimlox` will use that service-style ADC path
- if it is unset, `dimlox` will fall back to the standard local ADC file from `gcloud auth application-default login`

For developers switching back and forth, a tiny shell helper or env file is often
safer than trying to remember the current state by hand.

## Named gcloud profiles in dimlox

`dimlox` supports `--gcp-profile <name>` for commands that talk to GCS.

Example:

```bash
dimlox doctor --gcp-profile project-a
dimlox ls --gcp-profile project-a gs://example-bucket/
```

Important rule:

- a named gcloud profile changes identity only when its config includes `auth/credential_file_override`
- otherwise the profile contributes project/account context, but the underlying ADC identity stays whatever your process environment or default ADC file already provides

That distinction matters because `gcloud config configurations` and ADC are not
the same mechanism.

You can inspect the local profiles with:

```bash
dimlox doctor --list-gcp-profiles
```

Look for `credential_file_override` in the output to see which profiles are
identity-selecting versus context-only.

## Per-leg GCS auth for cp

`dimlox cp` resolves GCS auth per endpoint. This is the main workflow for cloud
to cloud copy when source and destination need different identities.

### Profile on one leg, explicit credentials on the other

```bash
dimlox cp \
  --gcp-profile-src project-a \
  --gcp-creds-file-dst /path/to/dest-service-account.json \
  "gs://source-bucket/data/orders.psv" \
  "gs://dest-bucket/data/orders.psv"
```

### Different credential files on both legs

```bash
dimlox cp \
  --gcp-creds-file-src /path/to/source-sa.json \
  --gcp-creds-file-dst /path/to/dest-sa.json \
  "gs://source-bucket/data/orders.psv" \
  "gs://dest-bucket/data/orders.psv"
```

Per-leg precedence for `cp` is:

1. `--gcp-creds-file-src` / `--gcp-creds-file-dst`
2. `--gcp-profile-src` / `--gcp-profile-dst`
3. global `--gcp-profile`
4. process `GOOGLE_APPLICATION_CREDENTIALS`
5. default ADC / metadata server

Per-leg project precedence is:

1. `--gcp-project-src` / `--gcp-project-dst`
2. global `--gcp-project`
3. profile `core/project`
4. `GCLOUD_PROJECT`
5. `GOOGLE_CLOUD_PROJECT`

## Project handling

`dimlox` uses `--gcp-project` for requester-pays buckets and defaults it from:

1. `GCLOUD_PROJECT`
2. `GOOGLE_CLOUD_PROJECT`

So a typical setup looks like:

```bash
export GCLOUD_PROJECT=<project-id>
gcloud auth application-default login
```

Or pass the flag explicitly:

```bash
dimlox doctor --gcp-project <project-id>
dimlox ls --gcp-project <project-id> gs://<bucket>/
```

Note: some user accounts can authenticate successfully but still fail to set a
quota project if they do not have `serviceusage.services.use` on that project.
That is one of the main reasons service-based auth is preferred for team use.

## Quick verification

```bash
dimlox doctor
dimlox ls --limit 5 gs://<bucket>/
dimlox doctor gs://<bucket>/<object>
```

Expected shape:

- `doctor` with no target should report `gcs: ok - ADC token acquired`
- `ls` should return bucket contents without buffering everything into memory
- targeted `doctor` should print metadata for an existing object

## Common failures

- `application default credentials not found`
  - run `gcloud auth application-default login`
- permission denied on bucket/object
  - auth exists, but the account lacks storage access for that resource
- requester-pays errors
  - pass `--gcp-project <project-id>` or export `GCLOUD_PROJECT`
- quota project cannot be set for user ADC
  - the user account may not have `serviceusage.services.use`; service-based auth
    is usually the better long-term setup here
- wrong or unset project
  - run `gcloud config get-value project` and confirm the expected billing project
