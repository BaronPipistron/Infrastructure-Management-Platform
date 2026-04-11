[CmdletBinding()]
param(
  [switch]$NoBuild,
  [switch]$NoDetach,
  [switch]$NoForceRecreate
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = (Resolve-Path (Join-Path $scriptDir "..\\..\\..")).Path
$composeFile = Join-Path $repoRoot "docker-compose.LocalE2E.yml"

$runtimeSshDir = Join-Path $repoRoot ".local-e2e\\.runtime\\ssh"
$privateKeyPath = Join-Path $runtimeSshDir "reconciler_local_e2e_key"
$publicKeyPath = Join-Path $runtimeSshDir "reconciler_local_e2e_key.pub"

New-Item -ItemType Directory -Force -Path $runtimeSshDir | Out-Null

Remove-Item -LiteralPath $privateKeyPath -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath $publicKeyPath -Force -ErrorAction SilentlyContinue

$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("imp-local-e2e-key-" + [Guid]::NewGuid().ToString("N"))
$tempPrivateKey = Join-Path $tempDir "id_ed25519"
New-Item -ItemType Directory -Force -Path $tempDir | Out-Null

try {
  $sshKeygen = (Get-Command ssh-keygen -ErrorAction Stop).Source
  $keygenArgs = '-q -t ed25519 -N "" -C imp-local-e2e-runtime -f "' + $tempPrivateKey + '"'
  $keygenStdOut = Join-Path $tempDir "ssh-keygen.stdout.log"
  $keygenStdErr = Join-Path $tempDir "ssh-keygen.stderr.log"
  $keygenProcess = Start-Process `
    -FilePath $sshKeygen `
    -ArgumentList $keygenArgs `
    -Wait `
    -PassThru `
    -NoNewWindow `
    -RedirectStandardOutput $keygenStdOut `
    -RedirectStandardError $keygenStdErr

  if (-not (Test-Path -LiteralPath $tempPrivateKey)) {
    throw "ssh-keygen did not produce private key file"
  }
  if ($keygenProcess.ExitCode -ne 0 -and $keygenProcess.ExitCode -ne 255) {
    $stderrText = ""
    if (Test-Path -LiteralPath $keygenStdErr) {
      $stderrText = (Get-Content -Raw -LiteralPath $keygenStdErr).Trim()
    }
    throw "ssh-keygen returned non-zero exit code: $($keygenProcess.ExitCode). $stderrText"
  }
  if ($keygenProcess.ExitCode -eq 255) {
    Write-Verbose "ssh-keygen returned 255 on Windows while writing .pub sidecar; deriving public key manually"
  }

  $privateContent = Get-Content -Raw -LiteralPath $tempPrivateKey
  Set-Content -LiteralPath $privateKeyPath -Value $privateContent -Encoding ascii -NoNewline

  $publicContent = & $sshKeygen -y -f $tempPrivateKey
  if ($LASTEXITCODE -ne 0) {
    throw "ssh-keygen -y returned non-zero exit code: $LASTEXITCODE"
  }
  Set-Content -LiteralPath $publicKeyPath -Value $publicContent -Encoding ascii -NoNewline
}
finally {
  Remove-Item -LiteralPath $tempDir -Recurse -Force -ErrorAction SilentlyContinue
}

$composeArgs = @("-f", $composeFile, "up")
if (-not $NoBuild) {
  $composeArgs += "--build"
}
if (-not $NoDetach) {
  $composeArgs += "-d"
}
if (-not $NoForceRecreate) {
  $composeArgs += "--force-recreate"
}

& docker compose @composeArgs
if ($LASTEXITCODE -ne 0) {
  throw "docker compose up failed with exit code $LASTEXITCODE"
}

Write-Output "Local E2E stand is up. Runtime SSH keys were generated in: $runtimeSshDir"
