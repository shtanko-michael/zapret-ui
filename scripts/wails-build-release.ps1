$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$manifestPath = Join-Path $root "build/windows/wails.exe.manifest"
$manifestRelease = Join-Path $root "build/windows/wails.exe.manifest.release"
$manifestBackup = Join-Path $root "build/windows/wails.exe.manifest.dev.bak"

if (!(Test-Path $manifestRelease)) {
  throw "Release manifest not found: $manifestRelease"
}

Copy-Item $manifestPath $manifestBackup -Force
Copy-Item $manifestRelease $manifestPath -Force

try {
  Write-Host "[INFO] Using release manifest (requireAdministrator)"
  Set-Location $root
  & wails build @args
} finally {
  Write-Host "[INFO] Restoring dev manifest"
  if (Test-Path $manifestBackup) {
    Copy-Item $manifestBackup $manifestPath -Force
    Remove-Item $manifestBackup -Force -ErrorAction SilentlyContinue
  }
}

