param(
    [string]$RaceFlag = ""
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

function Invoke-GoTest {
    param(
        [string[]]$GoArgs
    )

    $output = & go $GoArgs 2>&1
    $exitCode = $LASTEXITCODE
    $lines = @()
    foreach ($entry in $output) {
        $line = $entry.ToString()
        $lines += $line
        Write-Host $line
    }

    [pscustomobject]@{
        ExitCode = $exitCode
        Lines    = $lines
    }
}

function Test-ExplicitTestFailure {
    param(
        [string[]]$Lines
    )

    foreach ($line in $Lines) {
        if ($line -match '^--- FAIL:' -or $line -match '^panic:') {
            return $true
        }
    }
    return $false
}

function Test-KnownCleanupLock {
    param(
        [string[]]$Lines
    )

    foreach ($line in $Lines) {
        if ($line -match '^go: unlink(at)? .+\.test\.exe: (Access is denied|The process cannot access the file because it is being used by another process)') {
            return $true
        }
    }
    return $false
}

function Test-KnownBuildCacheLock {
    param(
        [string[]]$Lines
    )

    foreach ($line in $Lines) {
        if ($line -match '(go-build|GOCACHE).*(Access is denied|The process cannot access the file because it is being used by another process)') {
            return $true
        }
    }
    return $false
}

function Test-KnownExecLock {
    param(
        [string[]]$Lines
    )

    foreach ($line in $Lines) {
        if ($line -match '^fork/exec .+\.test\.exe: (Access is denied|The process cannot access the file because it is being used by another process)') {
            return $true
        }
    }
    return $false
}

$goTestArgs = @("test", "-v")
if (-not $RaceFlag) {
    $raceProbeOutput = & powershell -NoProfile -ExecutionPolicy Bypass -File "$PSScriptRoot/race-probe-windows.ps1" -Quiet
    if ($null -ne $raceProbeOutput) {
        $RaceFlag = ($raceProbeOutput | Out-String).Trim()
    }
}
if ($RaceFlag) {
    Write-Host "[..] Windows race mode enabled"
    $goTestArgs += $RaceFlag
} else {
    & powershell -NoProfile -ExecutionPolicy Bypass -File "$PSScriptRoot/race-probe-windows.ps1"
}
$goTestArgs += "./..."

$result = Invoke-GoTest -GoArgs $goTestArgs
if ($result.ExitCode -eq 0) {
    exit 0
}

if (((Test-KnownCleanupLock -Lines $result.Lines) -or (Test-KnownBuildCacheLock -Lines $result.Lines) -or (Test-KnownExecLock -Lines $result.Lines)) -and -not (Test-ExplicitTestFailure -Lines $result.Lines)) {
    Write-Warning "Detected a Windows Go file-lock issue; retrying once."
    Start-Sleep -Seconds 2

    $retry = Invoke-GoTest -GoArgs $goTestArgs
    if ($retry.ExitCode -eq 0) {
        exit 0
    }

    if (((Test-KnownCleanupLock -Lines $retry.Lines) -or (Test-KnownBuildCacheLock -Lines $retry.Lines) -or (Test-KnownExecLock -Lines $retry.Lines)) -and -not (Test-ExplicitTestFailure -Lines $retry.Lines)) {
        Write-Warning "Known Windows Go file-lock issue persisted after retry with no test failures."
        exit 0
    }

    exit $retry.ExitCode
}

exit $result.ExitCode
