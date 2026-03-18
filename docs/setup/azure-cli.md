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

## One-profile setup

If you only need one Azure login on the machine:

```bash
az login
az account show
```

After that, `dimlox doctor --az-profile ...` is not required. The default Azure CLI
login under `~/.azure` is enough.

## Named profile setup

Named profiles are useful when you switch between clients or tenants.

### Option A: use the local shell helper

On this machine we use an `az-profile` helper that sets `AZURE_CONFIG_DIR` to a
profile-specific directory under `~/.azure-profiles/`.

Example:

```bash
az-profile client-a
az login
az account show
```

Then run `dimlox` with the matching profile name:

```bash
dimlox doctor --az-profile client-a
dimlox ls --az-profile client-a azblob://<account>/<container>/
```

### Option B: do it manually

If you do not have the helper, set the profile directory yourself:

```bash
export AZURE_CONFIG_DIR="$HOME/.azure-profiles/client-a"
mkdir -p "$AZURE_CONFIG_DIR"
az login
az account show
```

You can also pin the subscription explicitly if needed:

```bash
az account set --subscription "<subscription-name-or-id>"
az account show
```

## How `dimlox` resolves `--az-profile`

When you pass `--az-profile <name>`, `dimlox` resolves the Azure CLI config dir in
this order:

1. `AZURE_PROFILES_DIR/<name>` if `AZURE_PROFILES_DIR` is set
2. `~/.azure-profiles/<name>` if that profile directory exists
3. `~/.azure/profiles/<name>` as a legacy fallback

This lets developers use either the local helper pattern or an older profile layout.

## Quick verification

```bash
dimlox doctor --az-profile client-a
dimlox ls --az-profile client-a --limit 5 azblob://<account>/<container>/
dimlox doctor --az-profile client-a azblob://<account>/<container>/<blob>
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
