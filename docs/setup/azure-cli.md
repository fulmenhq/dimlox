# Azure Blob Setup

`dimlox` uses `DefaultAzureCredential` for Azure Blob Storage.
For local development, that means the Azure CLI must be installed and logged in.

## What you need

- `az` installed
- a successful `az login` in the profile you want to use
- optional: a named profile layout so different client logins stay isolated

## Install Azure CLI

### Debian / Ubuntu

```bash
curl -sL https://aka.ms/InstallAzureCLIDeb | sudo bash
az version
```

### macOS (Homebrew)

```bash
brew update
brew install azure-cli
az version
```

### Windows

The recommended install method is `winget` (ships with Windows 11 and recent Windows 10):

```powershell
winget install Microsoft.AzureCLI
```

Close and reopen your terminal after install so `az` is on PATH.

Alternatives:

- **Scoop**: `scoop install azure-cli` (from the `main` bucket)
- **MSI installer**: download from [Microsoft's Azure CLI install page](https://learn.microsoft.com/en-us/cli/azure/install-azure-cli-windows) and run the `.msi`
- **Chocolatey**: `choco install azure-cli`

Verify:

```powershell
az version
```

The rest of this guide (login, profiles, `--az-profile`) works the same on Windows.
The only difference is path separators in `AZURE_CONFIG_DIR` — use backslashes or
forward slashes, both work with the Azure CLI on Windows:

```powershell
$env:AZURE_CONFIG_DIR = "$env:USERPROFILE\.azure-profiles\client-a"
az login
```

## One-profile setup

If you only need one Azure login on the machine:

```bash
az login
az account show
```

After that, `dimlox doctor --az-profile ...` is not required. The default Azure CLI
login under `~/.azure` is enough.

## Named profile setup

Named profiles are useful when you switch between clients or tenants. Each
profile is just an isolated Azure CLI config directory — `dimlox` picks it up
via `--az-profile <name>`.

### Create a named profile

Set `AZURE_CONFIG_DIR` to a new profile-specific directory, create it, and login:

**Windows (PowerShell):**

```powershell
$env:AZURE_CONFIG_DIR = "$env:USERPROFILE\.azure-profiles\e3-filemage"
New-Item -ItemType Directory -Force -Path $env:AZURE_CONFIG_DIR
az login
az account show
```

**Linux / macOS:**

```bash
export AZURE_CONFIG_DIR="$HOME/.azure-profiles/e3-filemage"
mkdir -p "$AZURE_CONFIG_DIR"
az login
az account show
```

Replace `e3-filemage` with whatever name makes sense for the account or client.

If the account has multiple subscriptions, pin the one you need:

```bash
az account set --subscription "<subscription-name-or-id>"
az account show
```

### Use the profile with dimlox

Once the profile exists and has a login, pass the name to any `dimlox` command:

```bash
dimlox doctor --az-profile e3-filemage
dimlox ls --az-profile e3-filemage azblob://<account>/<container>/
```

If `dimlox doctor --az-profile e3-filemage` says the profile is not logged in,
run the login again against that exact profile directory:

**Windows (PowerShell):**

```powershell
$env:AZURE_CONFIG_DIR = "$env:USERPROFILE\.azure-profiles\e3-filemage"
az login
```

**Linux / macOS:**

```bash
export AZURE_CONFIG_DIR="$HOME/.azure-profiles/e3-filemage"
az login
```

### Switching profiles

To switch, just set `AZURE_CONFIG_DIR` again in your current shell. Some teams
wrap this in a small shell helper or alias to avoid typing the full path each
time — but the underlying mechanism is always the same env var.

> **Future direction:** `dimlox doctor` already validates Azure auth. A natural
> extension would be to let `doctor` guide you through profile creation
> interactively when a named profile does not exist yet — eliminating the need
> for manual directory setup or external helper scripts.

## How `dimlox` resolves `--az-profile`

When you pass `--az-profile <name>`, `dimlox` resolves the Azure CLI config dir in
this order:

1. `AZURE_PROFILES_DIR/<name>` if `AZURE_PROFILES_DIR` is set
2. `~/.azure-profiles/<name>` if that profile directory exists
3. `~/.azure/profiles/<name>` as a legacy fallback

This lets developers use either the local helper pattern or an older profile layout.

## Quick verification

```bash
dimlox doctor --az-profile e3-filemage
dimlox ls --az-profile e3-filemage --limit 5 azblob://<account>/<container>/
dimlox doctor --az-profile e3-filemage azblob://<account>/<container>/<blob>
```

Expected shape:

- `doctor` with no target should report `azblob: ok - DefaultAzureCredential token acquired`
- `ls` should return real container contents
- targeted `doctor` should print metadata for an existing blob

## Common failures

- `Please run 'az login' to setup account`
  - the selected Azure CLI profile is not logged in yet
- `BlobNotFound`
  - auth is working, but the blob path is wrong or no longer exists
- wrong tenant or subscription
  - run `az account show` and `az account set --subscription ...` in the same profile
