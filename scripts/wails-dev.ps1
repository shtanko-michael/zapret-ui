$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$manifestDev = Join-Path $root "build/windows/wails.exe.manifest"

Write-Host "[INFO] Using dev manifest (asInvoker): $manifestDev"

Set-Location $root
& wails dev @args

