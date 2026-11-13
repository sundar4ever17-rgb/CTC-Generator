$ErrorActionPreference = 'Stop'
$projectDirectory = Split-Path -Parent $PSScriptRoot
Push-Location -LiteralPath $projectDirectory
try {
    docker compose up --build -d
    if ($LASTEXITCODE -ne 0) { throw 'Docker Compose failed. Check Docker Desktop and the build output.' }
    Write-Host 'CTC Generator: http://localhost:3020'
    Write-Host 'Demo C&B login: cb@demo.local / DemoCTC!2026 (unless DEMO_PASSWORD was changed).'
} finally {
    Pop-Location
}
