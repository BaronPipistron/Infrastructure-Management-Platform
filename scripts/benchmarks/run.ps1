[CmdletBinding()]
param(
  [Parameter(Position = 0)]
  [ValidateSet("all", "detection-scalability", "convergence", "reconcile-throughput", "partial-data", "cooldown")]
  [string]$Scenario = "all",

  [Parameter(Position = 1)]
  [string]$OutputRoot = ""
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = (Resolve-Path (Join-Path $scriptDir "..\\..")).Path
$harnessPath = Join-Path $repoRoot "tests\\benchmarks\\e2e\\harness.py"

$args = @($harnessPath, $Scenario)
if ($OutputRoot -ne "") {
  $args += @("--output-root", $OutputRoot)
}

& python @args
if ($LASTEXITCODE -ne 0) {
  throw "Benchmark harness failed with exit code $LASTEXITCODE"
}

