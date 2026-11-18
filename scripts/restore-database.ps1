[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$ServiceFile,
    [Parameter(Mandatory = $true)][string]$CertificateFile,
    [Parameter(Mandatory = $true)][string]$Archive,
    [Parameter(Mandatory = $true)][string]$ExpectedDatabase,
    [Parameter(Mandatory = $true)][switch]$ConfirmRestoreToEmptyDatabase,
    [string]$PostgresImage = 'postgres:18-alpine',
    [string]$DockerNetwork
)
$ErrorActionPreference = 'Stop'
if (-not $ConfirmRestoreToEmptyDatabase) { throw 'An explicit empty-database restore confirmation is required.' }
$servicePath = (Resolve-Path -LiteralPath $ServiceFile).Path
$certificatePath = (Resolve-Path -LiteralPath $CertificateFile).Path
$archivePath = (Resolve-Path -LiteralPath $Archive).Path
$checksumPath = "$archivePath.sha256"
if (-not (Test-Path -LiteralPath $checksumPath)) { throw 'A matching .sha256 file is required.' }
$expectedHash = ([IO.File]::ReadAllText($checksumPath).Trim() -split '\s+')[0]
if ((Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash -ine $expectedHash) { throw 'Archive checksum mismatch.' }
$common = @('run', '--rm', '--read-only', '--cap-drop', 'ALL', '--security-opt', 'no-new-privileges:true', '--mount', "type=bind,source=$servicePath,target=/run/secrets/pg_service.conf,readonly", '--mount', "type=bind,source=$certificatePath,target=/run/secrets/postgres_ca,readonly", '--mount', "type=bind,source=$archivePath,target=/backup/archive.dump,readonly", '-e', 'PGSERVICEFILE=/run/secrets/pg_service.conf')
if ($DockerNetwork) { $common += @('--network', $DockerNetwork) }
$actualDatabase = & docker @common $PostgresImage psql --dbname=service=ctc -X -A -t -v ON_ERROR_STOP=1 -c 'SELECT current_database()'
if ($LASTEXITCODE -ne 0 -or (($actualDatabase -join '').Trim() -cne $ExpectedDatabase)) { throw 'Target database identity does not match ExpectedDatabase.' }
$tableCount = & docker @common $PostgresImage psql --dbname=service=ctc -X -A -t -v ON_ERROR_STOP=1 -c "SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname NOT LIKE 'pg_toast%' AND c.relkind IN ('r','p','v','m','S','f')"
if ($LASTEXITCODE -ne 0 -or (($tableCount -join '').Trim() -ne '0')) { throw 'Restore refuses any target containing user tables, views or sequences. Provision a new isolated database.' }
& docker @common $PostgresImage pg_restore --dbname=service=ctc --single-transaction --exit-on-error --no-owner --no-acl /backup/archive.dump
if ($LASTEXITCODE -ne 0) { throw 'Restore failed and its transaction was rolled back.' }
Write-Host "Restore into $ExpectedDatabase completed. Reapply runtime grants and validate counts, audit history, revision calculations and stored PDFs before any cutover."
