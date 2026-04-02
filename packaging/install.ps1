#Requires -Version 5.1
<#
.SYNOPSIS
    Install jig-mcp on Windows.

.DESCRIPTION
    Installs jig-mcp to a single directory with bin/, tools/, scripts/, and .env.

.PARAMETER InstallRoot
    Root directory for jig-mcp installation (default: %LOCALAPPDATA%\jig-mcp).
    Binary goes to bin/, tools/scripts/config at root level.

.PARAMETER Uninstall
    Remove jig-mcp installation.

.EXAMPLE
    .\install.ps1
    .\install.ps1 -InstallRoot "C:\Tools\jig-mcp"
    .\install.ps1 -Uninstall
#>

param(
    [string]$InstallRoot = "$env:LOCALAPPDATA\jig-mcp",
    [switch]$Uninstall
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path

function Add-ToUserPath {
    param([string]$Dir)
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($currentPath -notlike "*$Dir*") {
        [Environment]::SetEnvironmentVariable("Path", "$currentPath;$Dir", "User")
        $env:Path = "$env:Path;$Dir"
        Write-Host "  Added $Dir to user PATH"
    } else {
        Write-Host "  $Dir is already in PATH"
    }
}

function Remove-FromUserPath {
    param([string]$Dir)
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $newPath = ($currentPath -split ";" | Where-Object { $_ -ne $Dir }) -join ";"
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    Write-Host "  Removed $Dir from user PATH"
}

if ($Uninstall) {
    Write-Host "Uninstalling jig-mcp..."
    if (Test-Path "$InstallRoot") {
        Remove-Item "$InstallRoot" -Recurse -Force
        Write-Host "  Removed $InstallRoot"
    }
    $BinDir = Join-Path $InstallRoot "bin"
    Remove-FromUserPath $BinDir
    Write-Host "Done."
    exit 0
}

# Check that the binary exists in the archive
$BinaryPath = Join-Path $ScriptDir "bin\jig-mcp.exe"
if (-not (Test-Path $BinaryPath)) {
    Write-Error "jig-mcp.exe not found in $ScriptDir\bin\. Make sure you extracted the full archive."
    exit 1
}

Write-Host "Installing jig-mcp..."
Write-Host "  Install root: $InstallRoot\"
Write-Host ""

# Create install root directory
New-Item -ItemType Directory -Force -Path $InstallRoot | Out-Null

# Install binary to bin/
$BinDir = Join-Path $InstallRoot "bin"
New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
Copy-Item $BinaryPath "$BinDir\jig-mcp.exe" -Force
Write-Host "  Installed binary to $BinDir\jig-mcp.exe"

# Install tools to tools/
$ToolsSrc = Join-Path $ScriptDir "tools"
$ToolsDst = Join-Path $InstallRoot "tools"
if (Test-Path $ToolsSrc) {
    Copy-Item $ToolsSrc $ToolsDst -Recurse
    Write-Host "  Installed example tools to $ToolsDst\"
}

# Install scripts to scripts/
$ScriptsSrc = Join-Path $ScriptDir "scripts"
$ScriptsDst = Join-Path $InstallRoot "scripts"
if (Test-Path $ScriptsSrc) {
    Copy-Item $ScriptsSrc $ScriptsDst -Recurse
    Write-Host "  Installed example scripts to $ScriptsDst\"
}

# Install .env config file
$EnvSrc = Join-Path $ScriptDir ".env"
$EnvDst = Join-Path $InstallRoot ".env"
if (Test-Path $EnvSrc) {
    Copy-Item $EnvSrc $EnvDst -Force
    Write-Host "  Installed config template to $EnvDst"
}

# Copy docs
$DocsSrc = Join-Path $ScriptDir "docs"
$DocsDst = Join-Path $InstallRoot "docs"
if (Test-Path $DocsSrc) {
    Copy-Item "$DocsSrc\*" $DocsDst -Recurse -Force
    Write-Host "  Installed docs to $DocsDst\"
}

# Create logs directory
$LogsDir = Join-Path $InstallRoot "logs"
New-Item -ItemType Directory -Force -Path $LogsDir | Out-Null

# Add bin/ to PATH
Add-ToUserPath $BinDir

Write-Host ""
Write-Host "Installation complete!"
Write-Host ""
Write-Host "Quick start:"
Write-Host "  cd $InstallRoot; .\bin\jig-mcp"
Write-Host ""
Write-Host "Or with SSE transport:"
Write-Host "  cd $InstallRoot; .\bin\jig-mcp -transport sse -port 3001"
Write-Host ""
Write-Host "Or run from anywhere (binary auto-detects config location):"
Write-Host "  jig-mcp"
Write-Host ""
Write-Host "Note: You may need to restart your terminal for PATH changes to take effect."
