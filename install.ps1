# ==============================================================================
# 🪐 SearXGo Windows PowerShell Installer Script
# https://github.com/BrianStovia/serxgo
# ==============================================================================

[CmdletBinding()]
param (
    [int]$Port = 8184,
    [string]$InstallPath = "$env:LOCALAPPDATA\SearXGo"
)

$ErrorActionPreference = "Stop"

Write-Host "==================================================================" -ForegroundColor Cyan
Write-Host "🪐 SearXGo - Windows Auto-Installer" -ForegroundColor Cyan
Write-Host "==================================================================" -ForegroundColor Cyan

# 1. Determine Target Directory
$BinDir = Join-Path $InstallPath "bin"
if (-not (Test-Path $BinDir)) {
    New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
}

$TargetExe = Join-Path $BinDir "searxgo.exe"
$TargetSettings = Join-Path $InstallPath "settings.yml"

Write-Host "➜ Target Installation Directory: $InstallPath" -ForegroundColor Yellow

# 2. Acquire Binary
Write-Host "➜ Deploying SearXGo executable..." -ForegroundColor Yellow

if (Test-Path ".\searxgo.exe") {
    Write-Host "✓ Copying local compiled searxgo.exe..." -ForegroundColor Green
    Copy-Item ".\searxgo.exe" -Destination $TargetExe -Force
} elseif (Test-Path ".\cmd\server\main.go" -and (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host "✓ Compiling searxgo.exe from source with Go..." -ForegroundColor Green
    go build -ldflags="-s -w" -o $TargetExe ./cmd/server
} else {
    Write-Host "➜ Downloading prebuilt Windows binary from GitHub..." -ForegroundColor Yellow
    $MainDistUrl = "https://raw.githubusercontent.com/BrianStovia/serxgo/main/dist/searxgo-windows-amd64.exe"
    $ReleaseUrl = "https://github.com/BrianStovia/serxgo/releases/latest/download/searxgo-windows-amd64.exe"
    $Downloaded = $false
    
    foreach ($Url in @($MainDistUrl, $ReleaseUrl)) {
        try {
            Invoke-WebRequest -Uri $Url -OutFile $TargetExe -UseBasicParsing -ErrorAction Stop
            if ((Get-Item $TargetExe).Length -gt 1000000) {
                Write-Host "✓ Prebuilt Windows binary downloaded successfully!" -ForegroundColor Green
                $Downloaded = $true
                break
            }
        } catch {
            # Continue to next URL
        }
    }

    if (-not $Downloaded) {
        Write-Warning "Could not download prebuilt binary directly. Building via Go if available..."
        if (Get-Command go -ErrorAction SilentlyContinue) {
            go install github.com/BrianStovia/serxgo/cmd/server@latest
            $Gopath = (go env GOPATH)
            Copy-Item "$Gopath\bin\server.exe" -Destination $TargetExe -Force
        } else {
            throw "Unable to acquire or build searxgo.exe. Please install Go or verify network connection."
        }
    }
}

# 3. Copy settings.yml
if (Test-Path ".\settings.yml") {
    Copy-Item ".\settings.yml" -Destination $TargetSettings -Force
} else {
    $SettingsUrl = "https://raw.githubusercontent.com/BrianStovia/serxgo/main/settings.yml"
    try {
        Invoke-WebRequest -Uri $SettingsUrl -OutFile $TargetSettings -UseBasicParsing
    } catch {
        Write-Warning "settings.yml could not be fetched from remote, defaults embedded in binary will be used."
    }
}

# 4. Add BinDir to User PATH Environment Variable if not present
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$BinDir*") {
    Write-Host "➜ Adding $BinDir to User PATH..." -ForegroundColor Yellow
    $NewPath = "$UserPath;$BinDir"
    [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
    $env:Path = "$env:Path;$BinDir"
    Write-Host "✓ PATH updated successfully!" -ForegroundColor Green
}

# 5. Create Start Shortcut (Optional helper script)
$StartScript = Join-Path $InstallPath "start-searxgo.bat"
@"
@echo off
title SearXGo Privacy Metasearch
cd /d "$InstallPath"
echo Starting SearXGo on http://localhost:$Port ...
"$TargetExe" -port $Port
pause
"@ | Set-Content -Path $StartScript -Encoding ASCII

Write-Host "==================================================================" -ForegroundColor Green
Write-Host "🎉 SearXGo successfully installed for Windows!" -ForegroundColor Green
Write-Host "➜ Binary Location: $TargetExe" -ForegroundColor White
Write-Host "➜ Web Interface:   http://localhost:$Port" -ForegroundColor Cyan
Write-Host "➜ Quick Launch:    Run 'searxgo' in any new PowerShell or Terminal" -ForegroundColor White
Write-Host "==================================================================" -ForegroundColor Green
