$ErrorActionPreference = 'Stop'
$projectDirectory = Split-Path -Parent $PSScriptRoot
Push-Location -LiteralPath $projectDirectory
try {
    docker compose stop
    if ($LASTEXITCODE -ne 0) { throw 'Docker Compose could not stop the demo.' }
} finally {
    Pop-Location
}
