[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$ServiceFile,
    [Parameter(Mandatory = $true)][string]$CertificateFile,
    [Parameter(Mandatory = $true)][string]$Destination,
    [string]$PostgresImage = 'postgres:18-alpine',
    [string]$DockerNetwork
)
$ErrorActionPreference = 'Stop'
$servicePath = (Resolve-Path -LiteralPath $ServiceFile).Path
$certificatePath = (Resolve-Path -LiteralPath $CertificateFile).Path
New-Item -ItemType Directory -Path $Destination -Force | Out-Null
$backupDirectory = (Resolve-Path -LiteralPath $Destination).Path
$stamp = [DateTime]::UtcNow.ToString('yyyyMMddTHHmmssZ')
$name = "ctc-$stamp-$([Guid]::NewGuid().ToString('N').Substring(0,8)).dump"
$common = @('run', '--rm', '--read-only', '--cap-drop', 'ALL', '--security-opt', 'no-new-privileges:true', '--mount', "type=bind,source=$servicePath,target=/run/secrets/pg_service.conf,readonly", '--mount', "type=bind,source=$certificatePath,target=/run/secrets/postgres_ca,readonly", '--mount', "type=bind,source=$backupDirectory,target=/backup", '-e', 'PGSERVICEFILE=/run/secrets/pg_service.conf')
if ($DockerNetwork) { $common += @('--network', $DockerNetwork) }
& docker @common $PostgresImage pg_dump --dbname=service=ctc --format=custom --no-owner --no-acl --file="/backup/$name"
if ($LASTEXITCODE -ne 0) { throw 'Backup failed. A partial archive may remain; it is not a verified backup.' }
& docker @common $PostgresImage pg_restore --list "/backup/$name" | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'Backup archive inspection failed. Do not use this archive for recovery.' }
$backupPath = Join-Path $backupDirectory $name
$digest = (Get-FileHash -LiteralPath $backupPath -Algorithm SHA256).Hash.ToLowerInvariant()
[IO.File]::WriteAllText("$backupPath.sha256", "$digest  $name`n", [Text.UTF8Encoding]::new($false))
Write-Host "Backup archive and SHA-256 written to $backupPath. Copy to encrypted off-host storage and test restoration."
