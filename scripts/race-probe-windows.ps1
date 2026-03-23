param(
    [switch]$Quiet
)

$ErrorActionPreference = "Continue"

function ConvertTo-ForwardSlashPath {
    param(
        [string]$Path
    )

    return ($Path -replace '\\', '/')
}

$repoRoot = ConvertTo-ForwardSlashPath (Split-Path -Parent $PSScriptRoot)
$tmpRoot = "$repoRoot/.tmp"
$env:GOCACHE = "$tmpRoot/go-build"
$env:GOTMPDIR = "$tmpRoot/go-tmp"

New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null
New-Item -ItemType Directory -Force -Path $env:GOTMPDIR | Out-Null

$cgoEnabled = (& go env CGO_ENABLED).Trim()
if ($LASTEXITCODE -ne 0 -or $cgoEnabled -ne "1") {
    if (-not $Quiet) {
        Write-Host ""
    }
    exit 0
}

$probeDir = Join-Path $tmpRoot "race-probe"
New-Item -ItemType Directory -Force -Path $probeDir | Out-Null
Set-Content -Path (Join-Path $probeDir "go.mod") -Value "module example.com/dimlox-race-probe`n"
Set-Content -Path (Join-Path $probeDir "race_probe_test.go") -Value @'
package raceprobe

import "testing"

func TestRaceProbe(t *testing.T) {}
'@

Push-Location $probeDir
try {
    $output = & go test -race -run TestRaceProbe . 2>&1
    $exitCode = $LASTEXITCODE
} finally {
    Pop-Location
}

if ($exitCode -eq 0) {
    if (-not $Quiet) {
        Write-Host "-race"
    }
    exit 0
}

if (-not $Quiet) {
    $cc = (& go env CC).Trim()
    $gcc = Get-Command gcc -ErrorAction SilentlyContinue
    $cl = Get-Command cl -ErrorAction SilentlyContinue
    $toolHints = @()
    if ($gcc) {
        $toolHints += "gcc on PATH"
    }
    if ($cl) {
        $toolHints += "cl.exe on PATH"
    }
    $toolHint = if ($toolHints.Count -gt 0) { $toolHints -join ", " } else { "no gcc/cl.exe detected on PATH" }
    Write-Warning "Skipping -race on Windows; CGO is enabled but the active shell is not race-ready (go env CC=$cc, $toolHint)."
}

exit 0
