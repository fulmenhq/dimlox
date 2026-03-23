param(
    [Parameter(Mandatory = $true, Position = 0)]
    [string]$Action,

    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$Args
)

$ErrorActionPreference = "Stop"

function ConvertTo-ForwardSlashPath {
    param(
        [string]$Path
    )

    return ($Path -replace '\\', '/')
}

function Get-RepoRoot {
    return ConvertTo-ForwardSlashPath (Split-Path -Parent $PSScriptRoot)
}

function Initialize-GoEnv {
    $repoRoot = Get-RepoRoot
    $tmpRoot = "$repoRoot/.tmp"
    $env:GOCACHE = "$tmpRoot/go-build"
    $env:GOTMPDIR = "$tmpRoot/go-tmp"

    New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null
    New-Item -ItemType Directory -Force -Path $env:GOTMPDIR | Out-Null
}

function Invoke-Go {
    param(
        [string[]]$GoArgs
    )

    Initialize-GoEnv
    & go $GoArgs
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
}

function Get-VersionValue {
    param(
        [string]$RepoRoot
    )

    $versionPath = Join-Path $RepoRoot "VERSION"
    if (-not (Test-Path $versionPath)) {
        return "dev"
    }
    return ((Get-Content $versionPath -Raw).Trim())
}

function Get-GitCommitValue {
    param(
        [string]$RepoRoot
    )

    $commit = & git -C $RepoRoot rev-parse --short HEAD 2>$null
    if ($LASTEXITCODE -ne 0 -or -not $commit) {
        return "unknown"
    }
    return $commit.Trim()
}

function Find-ToolPath {
    param(
        [string]$RepoRoot,
        [string]$ToolName
    )

    $candidates = @(
        (Join-Path $RepoRoot "bin\$ToolName.exe"),
        (Join-Path $RepoRoot "bin\$ToolName")
    )
    foreach ($candidate in $candidates) {
        if (Test-Path $candidate) {
            return (ConvertTo-ForwardSlashPath $candidate)
        }
    }

    $command = Get-Command $ToolName -ErrorAction SilentlyContinue
    if ($null -ne $command) {
        return (ConvertTo-ForwardSlashPath $command.Source)
    }

    return ""
}

function Invoke-GoBuild {
    param(
        [string]$Goos,
        [string]$Goarch,
        [string]$Ldflags,
        [string]$MainPath,
        [string]$OutputPath
    )

    Initialize-GoEnv
    $outputDir = Split-Path -Parent $OutputPath
    if ($outputDir) {
        New-Item -ItemType Directory -Force -Path $outputDir | Out-Null
    }

    $env:CGO_ENABLED = "0"
    $env:GOOS = $Goos
    $env:GOARCH = $Goarch
    & go build "-ldflags=$Ldflags" "-trimpath" "-o" $OutputPath $MainPath
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
}

function Invoke-Goneat {
    param(
        [string]$GoneatPath,
        [string[]]$GoneatArgs,
        [switch]$RequireTool
    )

    if (-not $GoneatPath) {
        if ($RequireTool) {
            Write-Host "[!!] goneat not found (run 'make bootstrap')"
            exit 1
        }
        return $false
    }

    & $GoneatPath $GoneatArgs
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
    return $true
}

function Show-MakeHelp {
    param(
        [string]$MakefilePath,
        [string]$Version
    )

    Write-Host "dimlox - dimension table manager and large-file cloud tool"
    Write-Host ""

    $entries = foreach ($line in Get-Content $MakefilePath) {
        if ($line -match '^([A-Za-z_-]+):.*##\s+(.*)$') {
            [pscustomobject]@{
                Target = $matches[1]
                Description = $matches[2]
            }
        }
    }

    foreach ($entry in $entries | Sort-Object Target) {
        Write-Host ("{0,-20} {1}" -f $entry.Target, $entry.Description)
    }

    Write-Host ""
    Write-Host "Current version: $Version"
}

function Write-VersionValue {
    param(
        [string]$RepoRoot,
        [string]$Version
    )

    $versionPath = Join-Path $RepoRoot "VERSION"
    [System.IO.File]::WriteAllText($versionPath, "$Version`n")
}

switch ($Action) {
    "go-env" {
        Initialize-GoEnv
        $value = & go env $Args[0]
        if ($LASTEXITCODE -ne 0) {
            exit $LASTEXITCODE
        }
        Write-Output $value.Trim()
    }

    "print-version" {
        Write-Output (Get-VersionValue $Args[0])
    }

    "print-build-time" {
        Write-Output ([DateTime]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ"))
    }

    "print-git-commit" {
        Write-Output (Get-GitCommitValue $Args[0])
    }

    "find-tool" {
        Write-Output (Find-ToolPath $Args[0] $Args[1])
    }

    "help" {
        Show-MakeHelp -MakefilePath $Args[0] -Version $Args[1]
    }

    "go" {
        Invoke-Go -GoArgs $Args
    }

    "build" {
        Invoke-GoBuild -Goos $Args[0] -Goarch $Args[1] -Ldflags $Args[2] -MainPath $Args[3] -OutputPath $Args[4]
        Write-Host "[ok] Built $($Args[4])"
    }

    "build-all" {
        $name = $Args[0]
        $ldflags = $Args[1]
        $mainPath = $Args[2]
        $targets = @(
            @{ Goos = "darwin"; Goarch = "amd64"; Output = "dist/release/$name-darwin-amd64" },
            @{ Goos = "darwin"; Goarch = "arm64"; Output = "dist/release/$name-darwin-arm64" },
            @{ Goos = "linux"; Goarch = "amd64"; Output = "dist/release/$name-linux-amd64" },
            @{ Goos = "linux"; Goarch = "arm64"; Output = "dist/release/$name-linux-arm64" },
            @{ Goos = "windows"; Goarch = "amd64"; Output = "dist/release/$name-windows-amd64.exe" },
            @{ Goos = "windows"; Goarch = "arm64"; Output = "dist/release/$name-windows-arm64.exe" }
        )

        foreach ($target in $targets) {
            Invoke-GoBuild -Goos $target.Goos -Goarch $target.Goarch -Ldflags $ldflags -MainPath $mainPath -OutputPath $target.Output
        }
        Write-Host "[ok] Built all targets to dist/release/"
    }

    "build-windows" {
        $ldflags = $Args[0]
        $mainPath = $Args[1]
        Invoke-GoBuild -Goos "windows" -Goarch "amd64" -Ldflags $ldflags -MainPath $mainPath -OutputPath ".tmp/build-check/dimlox-windows-amd64.exe"
        Invoke-GoBuild -Goos "windows" -Goarch "arm64" -Ldflags $ldflags -MainPath $mainPath -OutputPath ".tmp/build-check/dimlox-windows-arm64.exe"
        Write-Host "[ok] Windows cross-compiles passed"
    }

    "lint" {
        $goneatPath = $Args[0]
        if (-not (Invoke-Goneat -GoneatPath $goneatPath -GoneatArgs @("assess", "--categories", "lint", "--check"))) {
            Write-Host "[!!] goneat not found, falling back to go vet"
            Invoke-Go -GoArgs @("vet", "./...")
        }
    }

    "assess" {
        Invoke-Goneat -GoneatPath $Args[0] -GoneatArgs @("assess", "--categories", "format,lint,security", "--format", "concise") -RequireTool
    }

    "precommit" {
        $goneatPath = $Args[0]
        if (-not (Invoke-Goneat -GoneatPath $goneatPath -GoneatArgs @("assess", "--categories", "format,lint,security", "--fail-on", "critical"))) {
            Write-Host "[!!] goneat not found - skipping assess (run 'make bootstrap')"
        }
        Write-Host "[ok] Pre-commit checks passed"
    }

    "prepush" {
        Invoke-Goneat -GoneatPath $Args[0] -GoneatArgs @("assess", "--categories", "format,lint,security", "--fail-on", "high") -RequireTool
        Write-Host "[ok] Pre-push checks passed"
    }

    "tools" {
        if (Get-Command go -ErrorAction SilentlyContinue) {
            $goVersion = (& go version).Split(" ")[2]
            Write-Host "[ok] go: $goVersion"
        } else {
            Write-Host "[!!] go not found"
        }

        if ($Args[0]) {
            $goneatVersion = & $Args[0] version 2>&1 | Select-Object -First 1
            Write-Host "[ok] goneat: $goneatVersion"
        } else {
            Write-Host "[--] goneat: not found (optional, run 'make bootstrap')"
        }

        if (Get-Command git -ErrorAction SilentlyContinue) {
            $gitVersion = (& git --version).Split(" ")[2]
            Write-Host "[ok] git: $gitVersion"
        } else {
            Write-Host "[!!] git not found"
        }
    }

    "install" {
        $buildArtifact = $Args[0]
        $installTarget = $Args[1]
        $installDir = Split-Path -Parent $installTarget
        if ($installDir) {
            New-Item -ItemType Directory -Force -Path $installDir | Out-Null
        }
        Copy-Item -Force $buildArtifact $installTarget
        Write-Host "[ok] Installed to $installTarget"
    }

    "clean" {
        $paths = @("bin", "dist", "coverage.out", ".tmp/build-check")
        foreach ($path in $paths) {
            if (Test-Path $path) {
                Remove-Item -Recurse -Force $path
            }
        }
        Initialize-GoEnv
        & go clean -cache
        if ($LASTEXITCODE -ne 0) {
            exit $LASTEXITCODE
        }
        Write-Host "[ok] Cleaned"
    }

    "version-set" {
        if (-not $Args[1]) {
            Write-Host "usage: make version-set V=vX.Y.Z"
            exit 1
        }
        Write-VersionValue -RepoRoot $Args[0] -Version $Args[1]
        Write-Host "[ok] Version set to $($Args[1])"
    }

    "version-patch" {
        $repoRoot = $Args[0]
        $current = Get-VersionValue $repoRoot
        $parts = $current.TrimStart("v").Split(".")
        $newVersion = "v{0}.{1}.{2}" -f $parts[0], $parts[1], ([int]$parts[2] + 1)
        Write-VersionValue -RepoRoot $repoRoot -Version $newVersion
        Write-Host "[ok] $current -> $newVersion"
    }

    "version-minor" {
        $repoRoot = $Args[0]
        $current = Get-VersionValue $repoRoot
        $parts = $current.TrimStart("v").Split(".")
        $newVersion = "v{0}.{1}.0" -f $parts[0], ([int]$parts[1] + 1)
        Write-VersionValue -RepoRoot $repoRoot -Version $newVersion
        Write-Host "[ok] $current -> $newVersion"
    }

    "version-major" {
        $repoRoot = $Args[0]
        $current = Get-VersionValue $repoRoot
        $parts = $current.TrimStart("v").Split(".")
        $newVersion = "v{0}.0.0" -f ([int]$parts[0] + 1)
        Write-VersionValue -RepoRoot $repoRoot -Version $newVersion
        Write-Host "[ok] $current -> $newVersion"
    }

    default {
        Write-Error "unknown action: $Action"
        exit 1
    }
}
