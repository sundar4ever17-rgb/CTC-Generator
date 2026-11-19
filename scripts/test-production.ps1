# Creates only NEW, isolated synthetic test resources. Never uses a production URL.
[CmdletBinding()]
param(
    [string]$Image = 'ctc-generator-backend:production-upgrade',
    [int]$ApiPort = 8093,
    [switch]$KeepRunning
)
$ErrorActionPreference = 'Stop'
$projectDirectory = Split-Path -Parent $PSScriptRoot
Push-Location -LiteralPath $projectDirectory
$state = $null
$createdContainers = [Collections.Generic.List[string]]::new()
try {
    docker image inspect $Image --format '{{.Id}}'
    if ($LASTEXITCODE -ne 0) { throw 'Build the reviewed backend image first.' }
    if ($ApiPort -lt 1024 -or $ApiPort -gt 65535) { throw 'Use a non-privileged test port.' }
    $prefix = 'ctc-upgrade-test-prod-' + [DateTime]::UtcNow.ToString('yyyyMMddHHmmss') + '-' + [Guid]::NewGuid().ToString('N').Substring(0,4)
    $testRoot = Join-Path $projectDirectory "artifacts/$prefix"
    New-Item -ItemType Directory -Path $testRoot | Out-Null
    $state = @{ Prefix=$prefix; Network="$prefix-network"; Database="$prefix-db"; API="$prefix-api"; Volume="$prefix-pgdata"; Directory=$testRoot; Image=$Image; ApiPort=$ApiPort }
    $state | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $testRoot 'state.json')
    $encoding = [Text.UTF8Encoding]::new($false)
    $ownerPassword = [Guid]::NewGuid().ToString('N')
    $runtimePassword = [Guid]::NewGuid().ToString('N')
    $initialPassword = [Guid]::NewGuid().ToString('N')
    $enrolledPassword = [Guid]::NewGuid().ToString('N')
    $clusterPassword = [Guid]::NewGuid().ToString('N')
    [IO.File]::WriteAllText((Join-Path $testRoot 'cluster_password'), $clusterPassword, $encoding)
    docker run --rm --user 0 --mount "type=bind,source=$testRoot,target=/cert" --entrypoint openssl $Image req -x509 -newkey rsa:2048 -nodes -keyout /cert/server.key -out /cert/server.crt -days 2 -subj /CN=ctc-prod-test-db -addext subjectAltName=DNS:ctc-prod-test-db
    if ($LASTEXITCODE -ne 0) { throw 'Test certificate generation failed.' }
    docker network create $state.Network
    if ($LASTEXITCODE -ne 0) { throw 'Test network creation failed.' }
    docker volume create $state.Volume
    if ($LASTEXITCODE -ne 0) { throw 'Test volume creation failed.' }
    docker run -d --name $state.Database --network $state.Network --network-alias ctc-prod-test-db --mount "type=bind,source=$testRoot,target=/tls,readonly" --mount "type=volume,source=$($state.Volume),target=/var/lib/postgresql" -e POSTGRES_PASSWORD_FILE=/tls/cluster_password postgres:18-alpine sh -c 'cp /tls/server.crt /tmp/server.crt && cp /tls/server.key /tmp/server.key && chown postgres:postgres /tmp/server.key && chmod 600 /tmp/server.key && exec docker-entrypoint.sh postgres -c ssl=on -c ssl_cert_file=/tmp/server.crt -c ssl_key_file=/tmp/server.key'
    if ($LASTEXITCODE -ne 0) { throw 'Test PostgreSQL launch failed.' }
    $createdContainers.Add($state.Database)
    $ready = $false
    for ($attempt = 0; $attempt -lt 30; $attempt++) {
        docker exec $state.Database pg_isready -U postgres | Out-Null
        if ($LASTEXITCODE -eq 0) { $ready = $true; break }
        Start-Sleep -Seconds 1
    }
    if (-not $ready) { throw 'Test database did not become ready.' }
    $sql = @"
CREATE ROLE ctc_owner LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION PASSWORD '$ownerPassword';
CREATE ROLE ctc_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION PASSWORD '$runtimePassword';
CREATE DATABASE ctc_production_test OWNER ctc_owner;
CREATE DATABASE ctc_restore_test OWNER ctc_owner;
"@
    $sql | docker exec -i $state.Database psql -U postgres -v ON_ERROR_STOP=1
    if ($LASTEXITCODE -ne 0) { throw 'Test role setup failed.' }
    [IO.File]::WriteAllText((Join-Path $testRoot 'migrator_url'), "postgres://ctc_owner:${ownerPassword}@ctc-prod-test-db:5432/ctc_production_test?sslmode=verify-full&sslrootcert=/run/secrets/postgres_ca", $encoding)
    [IO.File]::WriteAllText((Join-Path $testRoot 'runtime_url'), "postgres://ctc_runtime:${runtimePassword}@ctc-prod-test-db:5432/ctc_production_test?sslmode=verify-full&sslrootcert=/run/secrets/postgres_ca", $encoding)
    [IO.File]::WriteAllText((Join-Path $testRoot 'bootstrap_password'), $initialPassword, $encoding)
    [IO.File]::WriteAllText((Join-Path $testRoot 'runner.env'), "CTC_TEST_FIXTURES=1`nTEST_PRODUCTION=1`nTEST_BASE_URL=http://$($state.API):8080`nTEST_ORIGIN=https://ctc.production.test`nTEST_ADMIN_EMAIL=admin@production.fixture`nTEST_INITIAL_PASSWORD=$initialPassword`nTEST_ADMIN_PASSWORD=$enrolledPassword`nARTIFACT_DIR=/artifacts`n", $encoding)
    $service = "[ctc]`nhost=ctc-prod-test-db`nport=5432`ndbname=ctc_production_test`nuser=ctc_owner`npassword=$ownerPassword`nsslmode=verify-full`nsslrootcert=/run/secrets/postgres_ca`n"
    [IO.File]::WriteAllText((Join-Path $testRoot 'backup-service.conf'), $service, $encoding)
    [IO.File]::WriteAllText((Join-Path $testRoot 'restore-service.conf'), $service.Replace('ctc_production_test','ctc_restore_test'), $encoding)
    & (Join-Path $PSScriptRoot 'backup-database.ps1') -ServiceFile (Join-Path $testRoot 'backup-service.conf') -CertificateFile (Join-Path $testRoot 'server.crt') -Destination (Join-Path $testRoot 'before-migration-backup') -DockerNetwork $state.Network
    $migration = @('run','--rm','--network',$state.Network,'--mount',"type=bind,source=$testRoot/server.crt,target=/run/secrets/postgres_ca,readonly",'--mount',"type=bind,source=$testRoot/migrator_url,target=/run/secrets/database_url,readonly",'-e','APP_ENV=production','-e','APP_ORIGIN=https://ctc.production.test','-e','DATABASE_URL_FILE=/run/secrets/database_url')
    & docker @migration $Image /app/ctc-api migrate
    if ($LASTEXITCODE -ne 0) { throw 'Migration command failed.' }
    Get-Content -Raw scripts/grant-runtime-access.sql | docker exec -i $state.Database psql -U ctc_owner -d ctc_production_test -v app_role=ctc_runtime -v ON_ERROR_STOP=1
    if ($LASTEXITCODE -ne 0) { throw 'Runtime grants failed.' }
    & docker @migration --mount "type=bind,source=$testRoot/bootstrap_password,target=/run/secrets/bootstrap_password,readonly" -e BOOTSTRAP_ADMIN_EMAIL=admin@production.fixture -e BOOTSTRAP_ADMIN_PASSWORD_FILE=/run/secrets/bootstrap_password $Image /app/ctc-api bootstrap-admin
    if ($LASTEXITCODE -ne 0) { throw 'Bootstrap command failed.' }
    $profile = Join-Path $PSScriptRoot 'security/chromium-seccomp.json'
    docker run -d --name $state.API --network $state.Network --user 10001:10001 --cap-drop ALL --security-opt no-new-privileges:true --security-opt "seccomp=$profile" --read-only --tmpfs /tmp:rw,nosuid,nodev,size=512m,mode=1777 --shm-size=256m --pids-limit=256 --memory=2g --cpus=2 --init --publish "127.0.0.1:${ApiPort}:8080" --mount "type=bind,source=$testRoot/server.crt,target=/run/secrets/postgres_ca,readonly" --mount "type=bind,source=$testRoot/runtime_url,target=/run/secrets/database_url,readonly" -e APP_ENV=production -e APP_ORIGIN=https://ctc.production.test -e DATABASE_URL_FILE=/run/secrets/database_url $Image
    if ($LASTEXITCODE -ne 0) { throw 'Production-mode test API launch failed.' }
    $createdContainers.Add($state.API)
    $ready = $false
    for ($attempt = 0; $attempt -lt 30; $attempt++) {
        try { Invoke-RestMethod "http://127.0.0.1:$ApiPort/api/health" | Out-Null; $ready = $true; break } catch { Start-Sleep -Seconds 1 }
    }
    if (-not $ready) { throw 'Production-mode test API did not become ready.' }
    $scriptsPath = Join-Path $projectDirectory 'scripts'
    $testsPath = Join-Path $projectDirectory 'tests'
    $runner = @('run','--rm','--network',$state.Network,'--env-file',"$testRoot/runner.env",'--mount',"type=bind,source=$testRoot,target=/artifacts",'--mount',"type=bind,source=$scriptsPath,target=/scripts,readonly",'--mount',"type=bind,source=$testsPath,target=/tests,readonly")
    & docker @runner node:24 node /scripts/production-smoke.mjs enroll
    if ($LASTEXITCODE -ne 0) { throw 'Required administrator enrollment failed.' }
    & docker @runner node:24 node /tests/security.mjs
    if ($LASTEXITCODE -ne 0) { throw 'Production security acceptance failed.' }
    & docker @runner node:24 node /scripts/production-smoke.mjs pdf
    if ($LASTEXITCODE -ne 0) { throw 'Production sandboxed PDF generation failed.' }
    & docker @runner node:24 node /tests/rules-api.mjs
    if ($LASTEXITCODE -ne 0) { throw 'Configurable rules, runtime grants, revision preservation or final documents failed.' }
    & (Join-Path $PSScriptRoot 'backup-database.ps1') -ServiceFile (Join-Path $testRoot 'backup-service.conf') -CertificateFile (Join-Path $testRoot 'server.crt') -Destination (Join-Path $testRoot 'populated-backup') -DockerNetwork $state.Network
    $archive = (Get-ChildItem -LiteralPath (Join-Path $testRoot 'populated-backup') -Filter '*.dump' | Select-Object -First 1).FullName
    & (Join-Path $PSScriptRoot 'restore-database.ps1') -ServiceFile (Join-Path $testRoot 'restore-service.conf') -CertificateFile (Join-Path $testRoot 'server.crt') -Archive $archive -ExpectedDatabase ctc_restore_test -ConfirmRestoreToEmptyDatabase -DockerNetwork $state.Network
    Get-Content -Raw scripts/grant-runtime-access.sql | docker exec -i $state.Database psql -U ctc_owner -d ctc_restore_test -v app_role=ctc_runtime -v ON_ERROR_STOP=1
    if ($LASTEXITCODE -ne 0) { throw 'Restored runtime grants failed.' }
    $query = "SELECT 'revisions',count(*) FROM ctc_revisions UNION ALL SELECT 'documents',count(*) FROM generated_documents UNION ALL SELECT 'audits',count(*) FROM audit_events UNION ALL SELECT 'migrations',count(*) FROM schema_migrations ORDER BY 1"
    $sourceCounts = docker exec $state.Database psql -U ctc_owner -d ctc_production_test -At -c $query
    if ($LASTEXITCODE -ne 0) { throw 'Source validation query failed.' }
    $restoreCounts = docker exec $state.Database psql -U ctc_owner -d ctc_restore_test -At -c $query
    if ($LASTEXITCODE -ne 0 -or ($sourceCounts -join ',') -ne ($restoreCounts -join ',')) { throw 'Restored data counts differ.' }
    $badChecksums = docker exec $state.Database psql -U ctc_owner -d ctc_restore_test -At -c "SELECT count(*) FROM generated_documents WHERE encode(sha256(content),'hex')<>checksum"
    if ($LASTEXITCODE -ne 0 -or (($badChecksums -join '').Trim() -ne '0')) { throw 'Restored document integrity check failed.' }
    [IO.File]::WriteAllText((Join-Path $testRoot 'recovery-results.json'), (@{ passed=$true; counts=$restoreCounts; recordedAt=[DateTime]::UtcNow.ToString('o') } | ConvertTo-Json), $encoding)
    Write-Host "Production-mode TLS, least-privilege runtime, secure cookies, RBAC, sandboxed PDFs and backup/restore checks passed. Evidence: $testRoot"
} finally {
    if ($createdContainers.Count -gt 0 -and -not $KeepRunning) {
        # Stop only containers created by this invocation; retain all test data.
        foreach ($container in $createdContainers) { docker stop $container | Out-Null }
    }
    Pop-Location
}
