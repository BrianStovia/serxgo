# ==============================================================================
# 🪐 SearXGo Windows PowerShell Update Script
# https://github.com/BrianStovia/serxgo
# ==============================================================================

[CmdletBinding()]
param (
    [string]$InstallPath = "$env:LOCALAPPDATA\SearXGo"
)

$ErrorActionPreference = "Stop"

Write-Host "==================================================================" -ForegroundColor Cyan
Write-Host "🪐 SearXGo - Windows Auto-Updater" -ForegroundColor Cyan
Write-Host "==================================================================" -ForegroundColor Cyan

$BinDir = Join-Path $InstallPath "bin"
$TargetExe = Join-Path $BinDir "searxgo.exe"

# If searxgo is in PATH elsewhere, detect it
$ExistingCmd = Get-Command searxgo -ErrorAction SilentlyContinue
if ($ExistingCmd) {
    $TargetExe = $ExistingCmd.Source
    $BinDir = Split-Path $TargetExe
    $InstallPath = Split-Path $BinDir
}

Write-Host "➜ Target Binary: $TargetExe" -ForegroundColor Yellow

# Backup existing binary
if (Test-Path $TargetExe) {
    $BackupName = "searxgo.exe.bak." + (Get-Date -Format "yyyyMMdd_HHmmss")
    $BackupPath = Join-Path $BinDir $BackupName
    Write-Host "➜ Creating backup at $BackupPath..." -ForegroundColor Cyan
    Copy-Item $TargetExe -Destination $BackupPath -Force
}

$TempDir = Join-Path $env:TEMP ("searxgo_update_" + [System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $TempDir -Force | Out-Null
$TempExe = Join-Path $TempDir "searxgo.exe"

try {
    # Check if running from git source repository with Go compiler
    if ((Test-Path ".\cmd\server\main.go") -and (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Host "✓ Updating git source and compiling latest binary..." -ForegroundColor Green
        git pull --ff-only origin main 2>$null
        go build -ldflags="-s -w" -o $TempExe ./cmd/server
    } else {
        Write-Host "➜ Fetching latest prebuilt release from GitHub..." -ForegroundColor Yellow
        $MainDistUrl = "https://raw.githubusercontent.com/BrianStovia/serxgo/main/dist/searxgo-windows-amd64.exe"
        $PrebuiltUrl = "https://raw.githubusercontent.com/BrianStovia/serxgo/prebuilt/dist/searxgo-windows-amd64.exe"
        $PrebuiltFallback = "https://raw.githubusercontent.com/BrianStovia/serxgo/prebuilt/searxgo-windows-amd64.exe"
        $ReleaseUrl = "https://github.com/BrianStovia/serxgo/releases/latest/download/searxgo-windows-amd64.exe"
        $Downloaded = $false

        foreach ($Url in @($MainDistUrl, $PrebuiltUrl, $PrebuiltFallback, $ReleaseUrl)) {
            try {
                Invoke-WebRequest -Uri $Url -OutFile $TempExe -UseBasicParsing -ErrorAction Stop
                if ((Get-Item $TempExe).Length -gt 1000000) {
                    Write-Host "✓ Prebuilt binary downloaded successfully!" -ForegroundColor Green
                    $Downloaded = $true
                    break
                }
            } catch {
                # Try next URL
            }
        }

        if (-not $Downloaded) {
            Write-Warning "Direct binary download failed. Attempting Go build..."
            if (Get-Command go -ErrorAction SilentlyContinue) {
                go install github.com/BrianStovia/serxgo/cmd/server@latest
                $GoBin = Join-Path (go env GOPATH) "bin\server.exe"
                if (Test-Path $GoBin) {
                    Copy-Item $GoBin -Destination $TempExe -Force
                    $Downloaded = $true
                }
            }
        }
    }

    if (-not (Test-Path $TempExe)) {
        throw "Failed to download or compile the latest SearXGo binary."
    }

    # Stop any currently running searxgo processes
    Get-Process -Name "searxgo" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    Start-Sleep -Milliseconds 500

    # Install updated binary
    Copy-Item $TempExe -Destination $TargetExe -Force
    Write-Host "✓ Installed updated binary to $TargetExe" -ForegroundColor Green

    Write-Host ""
    Write-Host "==================================================================" -ForegroundColor Green
    Write-Host "🎉 SearXGo updated successfully!" -ForegroundColor Green
    Write-Host "➜ Binary: $TargetExe" -ForegroundColor Cyan
    Write-Host "==================================================================" -ForegroundColor Green
}
finally {
    if (Test-Path $TempDir) {
        Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}
