[CmdletBinding()]
param(
  [switch]$RemoveVolumes,
  [switch]$KeepRuntimeKeys
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = (Resolve-Path (Join-Path $scriptDir "..\\..\\..")).Path
$composeFile = Join-Path $repoRoot "docker-compose.LocalE2E.yml"

$composeArgs = @("-f", $composeFile, "down")
if ($RemoveVolumes) {
  $composeArgs += "--volumes"
}

& docker compose @composeArgs
if ($LASTEXITCODE -ne 0) {
  throw "docker compose down failed with exit code $LASTEXITCODE"
}

if (-not $KeepRuntimeKeys) {
  $runtimeSshDir = Join-Path $repoRoot ".local-e2e\\.runtime\\ssh"
  Remove-Item -LiteralPath $runtimeSshDir -Recurse -Force -ErrorAction SilentlyContinue
  Write-Output "Local E2E stand is down. Runtime SSH artifacts removed."
}
else {
  Write-Output "Local E2E stand is down. Runtime SSH artifacts were preserved."
}
