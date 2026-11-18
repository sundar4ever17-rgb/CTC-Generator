[CmdletBinding()]
param([Parameter(Mandatory = $true)][string]$Image)
$ErrorActionPreference = 'Stop'
$projectDirectory = Split-Path -Parent $PSScriptRoot
$profile = Join-Path $PSScriptRoot 'security/chromium-seccomp.json'
Push-Location -LiteralPath $projectDirectory
try {
    docker run --rm --network none --user 10001:10001 --cap-drop ALL --security-opt no-new-privileges:true --security-opt "seccomp=$profile" --read-only --tmpfs /tmp:rw,nosuid,nodev,size=128m -e HOME=/tmp --entrypoint /usr/bin/chromium $Image --headless --disable-gpu --disable-dev-shm-usage --user-data-dir=/tmp/ctc-sandbox-check --dump-dom about:blank
    if ($LASTEXITCODE -ne 0) { throw 'Chromium sandbox preflight failed. Check host user namespace/AppArmor support and the reviewed seccomp profile. Do not disable the sandbox.' }
    Write-Host 'Chromium started successfully with its sandbox and production container restrictions.'
} finally {
    Pop-Location
}
