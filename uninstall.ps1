# ==============================================================================
# SearXGo Windows PowerShell Uninstaller Script
# https://github.com/BrianStovia/serxgo
# ==============================================================================

[CmdletBinding()]
param (
    [string]$InstallPath = "$env:LOCALAPPDATA\SearXGo",
    [switch]$Purge,
    [switch]$Force
)

$ErrorActionPreference = "Stop"

Write-Host "==================================================================" -ForegroundColor Cyan
Write-Host " SearXGo - Windows Auto-Uninstaller" -ForegroundColor Cyan
Write-Host "==================================================================" -ForegroundColor Cyan

# 1. User confirmation if not forced
if (-not $Force) {
    $Confirm = Read-Host "Are you sure you want to uninstall SearXGo? (y/N)"
    if ($Confirm -notmatch "^[yY]([eE][sS])?$") {
        Write-Host "Uninstallation canceled." -ForegroundColor Red
        return
    }

    if (-not $Purge) {
        $PurgeConfirm = Read-Host "Do you also want to delete configuration (settings.yml) and all data? (y/N)"
        if ($PurgeConfirm -match "^[yY]([eE][sS])?$") {
            $Purge = $true
        }
    }
}

# 2. Stop running searxgo processes
Write-Host "-> Stopping any running SearXGo processes..." -ForegroundColor Yellow
$Running = Get-Process -Name "searxgo" -ErrorAction SilentlyContinue
if ($Running) {
    $Running | Stop-Process -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 1
    Write-Host "[OK] Stopped running searxgo process." -ForegroundColor Green
}

# 3. Detect Installation Path
$BinDir = Join-Path $InstallPath "bin"
$ExistingCmd = Get-Command "searxgo" -ErrorAction SilentlyContinue
if ($ExistingCmd) {
    $DetectedExe = $ExistingCmd.Source
    $DetectedBin = Split-Path $DetectedExe
    $DetectedRoot = Split-Path $DetectedBin
    if (Test-Path $DetectedRoot) {
        $InstallPath = $DetectedRoot
        $BinDir = $DetectedBin
    }
}

Write-Host "-> Target Directory: $InstallPath" -ForegroundColor Yellow

# 4. Remove BinDir from User PATH
Write-Host "-> Removing $BinDir from User PATH..." -ForegroundColor Yellow
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -like "*$BinDir*") {
    $PathParts = $UserPath -split ";" | Where-Object { $_ -and ($_ -ne $BinDir) }
    $NewPath = [string]::Join(";", $PathParts)
    [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
    $CurrentParts = $env:Path -split ";" | Where-Object { $_ -and ($_ -ne $BinDir) }
    $env:Path = [string]::Join(";", $CurrentParts)
    Write-Host "[OK] Removed from User PATH." -ForegroundColor Green
}

# 5. Delete Files and Directories
if (Test-Path $InstallPath) {
    if ($Purge) {
        Write-Host "-> Purging entire installation directory ($InstallPath)..." -ForegroundColor Yellow
        Remove-Item -Path $InstallPath -Recurse -Force -ErrorAction SilentlyContinue
        Write-Host "[OK] Purged all SearXGo files and settings." -ForegroundColor Green
    } else {
        Write-Host "-> Removing binary and runner scripts..." -ForegroundColor Yellow
        if (Test-Path $BinDir) {
            Remove-Item -Path $BinDir -Recurse -Force -ErrorAction SilentlyContinue
        }
        $StartBat = Join-Path $InstallPath "start-searxgo.bat"
        if (Test-Path $StartBat) {
            Remove-Item -Path $StartBat -Force -ErrorAction SilentlyContinue
        }
        Write-Host "[OK] Binary removed." -ForegroundColor Green
        Write-Host "[INFO] Configuration retained at: $InstallPath\settings.yml" -ForegroundColor Yellow
    }
} else {
    Write-Host "[INFO] Installation folder $InstallPath not found." -ForegroundColor Yellow
}

Write-Host ""
Write-Host "==================================================================" -ForegroundColor Green
Write-Host " SearXGo has been uninstalled successfully from Windows." -ForegroundColor Green
Write-Host "==================================================================" -ForegroundColor Green
