[CmdletBinding()]
param(
    [int]$BuildCacheUntilHours = 168,
    [string]$BuildCacheMaxUsedSpace = "20GB",
    [int]$CompactVhdxAboveGb = 60,
    [int]$CompactWhenFreeBelowGb = 80,
    [int]$DockerWaitSeconds = 180,
    [switch]$SkipCompact
)

$ErrorActionPreference = "Continue"

$logRoot = Join-Path $env:LOCALAPPDATA "DockerDiskMaintenance"
New-Item -ItemType Directory -Force -Path $logRoot | Out-Null
$logPath = Join-Path $logRoot "maintenance.log"

Start-Transcript -Path $logPath -Append | Out-Null

function ConvertTo-Gb {
    param([long]$Bytes)
    [math]::Round($Bytes / 1GB, 2)
}

function Get-CDriveInfo {
    Get-CimInstance Win32_LogicalDisk -Filter "DeviceID='C:'"
}

function Test-DockerReady {
    docker info *> $null
    return $LASTEXITCODE -eq 0
}

function Wait-DockerReady {
    param([int]$TimeoutSeconds)

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        if (Test-DockerReady) {
            return $true
        }
        Start-Sleep -Seconds 3
    }
    return $false
}

function Write-DiskSummary {
    param([string]$Label)

    $disk = Get-CDriveInfo
    Write-Host ("{0}: C: free={1:N2}GB used={2:N2}GB total={3:N2}GB" -f `
        $Label, (ConvertTo-Gb $disk.FreeSpace), (ConvertTo-Gb ($disk.Size - $disk.FreeSpace)), (ConvertTo-Gb $disk.Size))
}

try {
    Write-Host ("==== Docker disk maintenance started: {0} ====" -f (Get-Date -Format "yyyy-MM-dd HH:mm:ss"))
    Write-DiskSummary "Before"

    $dockerDesktopExe = "C:\Program Files\Docker\Docker\Docker Desktop.exe"
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
        Write-Host "Docker CLI was not found. Exiting."
        exit 0
    }

    $initialDockerReady = Test-DockerReady
    $startedByScript = $false

    if (-not $initialDockerReady) {
        if (-not (Test-Path -LiteralPath $dockerDesktopExe)) {
            Write-Host "Docker Desktop executable was not found. Exiting."
            exit 0
        }

        Write-Host "Starting Docker Desktop..."
        docker desktop start
        $startedByScript = $true

        if (-not (Wait-DockerReady -TimeoutSeconds $DockerWaitSeconds)) {
            Write-Host "Docker daemon did not become ready. Exiting without cleanup."
            exit 1
        }
    }

    Write-Host "Docker daemon is ready."
    Write-Host "Pruning Docker build cache..."
    docker builder prune -af --filter "until=$($BuildCacheUntilHours)h" --max-used-space $BuildCacheMaxUsedSpace

    Write-Host "Pruning unused Docker objects, preserving volumes..."
    docker system prune -af --filter "until=$($BuildCacheUntilHours)h"

    Write-Host "Docker usage after prune:"
    docker system df

    Write-Host "Running fstrim inside docker-desktop..."
    wsl -d docker-desktop -u root -- sh -lc "fstrim -av || true; sync"

    $vhdx = Join-Path $env:LOCALAPPDATA "Docker\wsl\disk\docker_data.vhdx"
    $diskAfterPrune = Get-CDriveInfo
    $freeGb = ConvertTo-Gb $diskAfterPrune.FreeSpace
    $vhdxGb = 0
    if (Test-Path -LiteralPath $vhdx) {
        $vhdxGb = ConvertTo-Gb (Get-Item -LiteralPath $vhdx).Length
    }

    Write-Host ("Docker VHDX size after prune/fstrim: {0:N2}GB" -f $vhdxGb)

    $shouldCompact = (-not $SkipCompact) -and (Test-Path -LiteralPath $vhdx) -and `
        (($vhdxGb -ge $CompactVhdxAboveGb) -or ($freeGb -lt $CompactWhenFreeBelowGb))

    if ($shouldCompact) {
        Write-Host "Compacting Docker VHDX. Docker Desktop will be stopped temporarily."
        docker desktop stop
        Start-Sleep -Seconds 5
        wsl --shutdown
        Start-Sleep -Seconds 3

        $diskpartScript = @"
select vdisk file="$vhdx"
attach vdisk readonly
compact vdisk
detach vdisk
exit
"@
        $scriptPath = Join-Path $env:TEMP "compact-docker-data-vhdx.diskpart"
        Set-Content -LiteralPath $scriptPath -Value $diskpartScript -Encoding ASCII

        diskpart /s $scriptPath
        $diskpartExitCode = $LASTEXITCODE
        Remove-Item -LiteralPath $scriptPath -Force -ErrorAction SilentlyContinue
        Write-Host ("diskpart exit code: {0}" -f $diskpartExitCode)

        if ($initialDockerReady -or $startedByScript) {
            Write-Host "Starting Docker Desktop after compact..."
            docker desktop start
            [void](Wait-DockerReady -TimeoutSeconds $DockerWaitSeconds)
        }
    }
    elseif ($startedByScript -and -not $initialDockerReady) {
        Write-Host "Docker was started only for maintenance; stopping it again."
        docker desktop stop
    }
    else {
        Write-Host "Skipping VHDX compact. Threshold not reached or SkipCompact was set."
    }

    if (Test-Path -LiteralPath $vhdx) {
        $finalVhdxGb = ConvertTo-Gb (Get-Item -LiteralPath $vhdx).Length
        Write-Host ("Final Docker VHDX size: {0:N2}GB" -f $finalVhdxGb)
    }
    Write-DiskSummary "After"
    Write-Host ("==== Docker disk maintenance finished: {0} ====" -f (Get-Date -Format "yyyy-MM-dd HH:mm:ss"))
}
finally {
    Stop-Transcript | Out-Null
}
