# Scout installer for Windows.
# Usage: irm https://raw.githubusercontent.com/inovacc/scout/main/scripts/install.ps1 | iex
$ErrorActionPreference = 'Stop'

$Repo = "inovacc/scout"
$AppDir = if ($env:SCOUT_INSTALL_DIR) { $env:SCOUT_INSTALL_DIR } else { "$env:LOCALAPPDATA\Scout" }

# Detect architecture.
$Arch = if ([Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }
} else {
    Write-Error "32-bit Windows is not supported."
    exit 1
}

# Get latest release tag.
Write-Host "Fetching latest release..."
$Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
$Tag = $Release.tag_name
if (-not $Tag) {
    Write-Error "Failed to determine latest release."
    exit 1
}
$Version = $Tag.TrimStart('v')
Write-Host "Latest release: $Tag"

# Download archive.
$Asset = "scout_${Version}_windows_${Arch}.zip"
$Url = "https://github.com/$Repo/releases/download/$Tag/$Asset"

$TmpDir = Join-Path $env:TEMP "scout-install-$(Get-Random)"
New-Item -ItemType Directory -Path $TmpDir -Force | Out-Null

try {
    Write-Host "Downloading $Url..."
    Invoke-WebRequest -Uri $Url -OutFile (Join-Path $TmpDir $Asset)

    Write-Host "Extracting..."
    $savedPref = $ErrorActionPreference
    $ErrorActionPreference = 'SilentlyContinue'
    & "$env:SystemRoot\System32\tar.exe" -xf (Join-Path $TmpDir $Asset) -C $TmpDir 2>&1 | Out-Null
    $ErrorActionPreference = $savedPref

    # Install binary to app dir.
    New-Item -ItemType Directory -Path $AppDir -Force | Out-Null
    Copy-Item -Path (Join-Path $TmpDir "scout.exe") -Destination (Join-Path $AppDir "scout.exe") -Force

    # Add app dir to PATH for CLI access.
    $UserPath = [Environment]::GetEnvironmentVariable('PATH', 'User')
    if ($UserPath -notlike "*$AppDir*") {
        [Environment]::SetEnvironmentVariable('PATH', "$UserPath;$AppDir", 'User')
        $env:PATH = "$env:PATH;$AppDir"
        Write-Host "Added $AppDir to user PATH."
    }

    Write-Host ""
    Write-Host "Installed scout $Tag"
    Write-Host "  Binary: $AppDir\scout.exe"
    Write-Host ""
    Write-Host "Run 'scout setup' to configure your AI coding assistant."
}
finally {
    Remove-Item -Path $TmpDir -Recurse -Force -ErrorAction SilentlyContinue
}
