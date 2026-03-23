# Windows Setup

This guide covers getting `dimlox` and its cloud provider dependencies running on
Windows. Each provider's full auth and profile configuration is documented
separately — this page focuses on installation and Windows-specific details.

## Prerequisites

- Windows 10 (1809+) or Windows 11
- Go 1.23+ ([download](https://go.dev/dl/) or `scoop install go`)
- Git ([download](https://git-scm.com/download/win), `winget install Git.Git`, or `scoop install git`)
- GNU make (`winget install -e --id GnuWin32.Make` or `scoop install make`)

After `make` is available, you can install or verify the rest of the developer
tooling with:

```powershell
make bootstrap
make tools
```

## Build and install dimlox

```powershell
git clone https://github.com/fulmenhq/dimlox.git
cd dimlox
make build
make install
```

`make install` places the binary at `%APPDATA%\dimlox\bin\dimlox.exe`.

Add this to your PATH if it is not already there:

```powershell
# Current session
$env:PATH += ";$env:APPDATA\dimlox\bin"

# Permanent (user-level)
[Environment]::SetEnvironmentVariable("PATH",
    [Environment]::GetEnvironmentVariable("PATH", "User") + ";$env:APPDATA\dimlox\bin",
    "User")
```

Verify:

```powershell
dimlox version
```

## Cloud CLI tools

`dimlox` delegates authentication to the standard cloud CLIs. Install whichever
providers you need.

### Azure CLI

Required for Azure Blob Storage (`azblob://`) operations.

```powershell
winget install Microsoft.AzureCLI
```

Alternatives: `scoop install azure-cli`, MSI installer, `choco install azure-cli`.

Full setup: [`docs/setup/azure-cli.md`](azure-cli.md)

### Google Cloud SDK

Required for Google Cloud Storage (`gs://`) operations.

Download the installer from
[Google's Cloud SDK install page](https://cloud.google.com/sdk/docs/install#windows),
or use a package manager:

```powershell
scoop bucket add extras
scoop install gcloud
# or
choco install gcloudsdk
```

Full setup: [`docs/setup/gcloud-storage.md`](gcloud-storage.md)

### AWS CLI (future)

`dimlox` does not have an S3 provider yet, but if your workflow involves AWS
alongside Azure or GCS, install the AWS CLI now so it is ready:

```powershell
winget install Amazon.AWSCLI
```

Alternatives: `scoop install aws`, MSI from [AWS CLI install page](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html).

## Terminal recommendations

`dimlox` detects TTY capability and adjusts its progress output accordingly.

| Terminal | ANSI support | Progress display |
|----------|-------------|-----------------|
| Windows Terminal | Full | ANSI progress bars and colours |
| PowerShell 7+ (pwsh) | Full | ANSI progress bars and colours |
| PowerShell 5.1 (default) | Partial | Works in Windows Terminal; plain text in legacy console |
| cmd.exe | Limited | Plain text progress (no escape sequences) |
| Piped / non-TTY | N/A | Structured JSON lines on stderr |

For the best experience, use **Windows Terminal** with **PowerShell 7** or the
built-in PowerShell.

## Windows-specific behaviour

### Path lengths

`dimlox split` preflights all planned output paths against the 260-character
Windows `MAX_PATH` limit before writing any files. If a shard path would exceed
the limit, the operation fails with an actionable error suggesting a shorter
`--out-dir`. This check also runs in `--dry-run` mode.

### Atomic writes

Split shard writes use `.part` temporary files with atomic rename on completion.
On Windows, `dimlox` handles the case where the target already exists (Windows
`os.Rename` does not overwrite) by removing the target before renaming.

### Manifest portability

Manifest `shard_file` paths always use forward slashes regardless of the host OS.
A manifest written on Windows is readable on Linux and vice versa.

## Quick smoke test (local files only)

No cloud credentials needed — this validates the build and local provider:

```powershell
# Create a small test file
"col1|col2`n1|2`n3|4`n5|6`n7|8`n" | Out-File -Encoding utf8 -NoNewline test.psv

# Inspect
dimlox inspect --detect test.psv
dimlox inspect --wc test.psv
dimlox inspect --head 3 test.psv

# Split
dimlox split --rows 2 --header --manifest --out-dir shards test.psv

# Check results
Get-ChildItem shards
Get-Content shards\test_shard_0001.psv

# Clean up
Remove-Item test.psv
Remove-Item -Recurse shards
```
