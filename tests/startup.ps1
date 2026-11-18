param(
  [string]$Image = 'ctc-generator-backend:production-upgrade',
  [string]$ArtifactDirectory = (Join-Path $PSScriptRoot '..\artifacts\production')
)
# These probes use no network, ports, mounts, or database. All credentials below
# are synthetic values used solely to verify fail-closed startup validation.
$ErrorActionPreference = 'Continue'
$baseline = @{
  APP_ENV = 'production'
  APP_ORIGIN = 'https://ctc-test.invalid'
  DATABASE_URL = 'postgres://fixture:fixture@127.0.0.1/ctc_test?sslmode=verify-full'
}
$checks = @(
  @{ Name = 'explicit environment required'; Values = @{ APP_ENV = 'development' }; Pattern = 'APP_ENV must explicitly' },
  @{ Name = 'HTTPS origin required'; Values = @{ APP_ORIGIN = 'http://ctc-test.invalid' }; Pattern = 'HTTPS origin' },
  @{ Name = 'origin paths rejected'; Values = @{ APP_ORIGIN = 'https://ctc-test.invalid/path' }; Pattern = 'HTTPS origin' },
  @{ Name = 'database TLS verification required'; Values = @{ DATABASE_URL = 'postgres://fixture:fixture@127.0.0.1/ctc_test?sslmode=disable' }; Pattern = 'sslmode=verify-full' },
  @{ Name = 'demo credentials rejected'; Values = @{ DEMO_PASSWORD = 'SyntheticTestPassword!2026' }; Pattern = 'demo or bootstrap credentials' },
  @{ Name = 'bootstrap credentials excluded from runtime'; Values = @{ BOOTSTRAP_ADMIN_PASSWORD = 'SyntheticTestPassword!2026' }; Pattern = 'demo or bootstrap credentials' },
  @{ Name = 'conflicting secret sources rejected'; Values = @{ DATABASE_URL_FILE = '/missing-secret' }; Pattern = 'set only DATABASE_URL or DATABASE_URL_FILE' },
  @{ Name = 'unsafe pool size rejected'; Values = @{ DB_POOL_MAX_CONNS = '0' }; Pattern = 'DB_POOL_MAX_CONNS must be' }
)
$outcomes = foreach ($check in $checks) {
  $environment = $baseline.Clone()
  foreach ($key in $check.Values.Keys) { $environment[$key] = $check.Values[$key] }
  $arguments = @('run', '--rm', '--network', 'none')
  foreach ($key in $environment.Keys) { $arguments += @('-e', "$key=$($environment[$key])") }
  $arguments += $Image
  $output = (& docker @arguments 2>&1 | Out-String)
  $passed = $LASTEXITCODE -ne 0 -and $output -match $check.Pattern -and $output -notmatch 'SyntheticTestPassword!2026'
  if ($passed) { Write-Host "PASS $($check.Name)" } else { Write-Host "FAIL $($check.Name): $output" }
  @{ name = $check.Name; passed = $passed }
}
New-Item -ItemType Directory -Path $ArtifactDirectory -Force | Out-Null
@{ at = [DateTime]::UtcNow.ToString('o'); outcomes = @($outcomes) } | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $ArtifactDirectory 'startup-results.json') -Encoding UTF8
if (@($outcomes | Where-Object { -not $_.passed }).Count -gt 0) { exit 1 }
