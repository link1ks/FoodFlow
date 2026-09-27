param([ValidateSet('up','restart','status','backup','restore-check')][string]$Action='status')
$ErrorActionPreference='Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
$config=Join-Path $PWD '.cache/acceptance.env'
New-Item -ItemType Directory -Force .cache | Out-Null
if(!(Test-Path -LiteralPath $config)){
  $password=[Convert]::ToHexString([Security.Cryptography.RandomNumberGenerator]::GetBytes(24))
  $secret=[Convert]::ToHexString([Security.Cryptography.RandomNumberGenerator]::GetBytes(48))
  "POSTGRES_PASSWORD=$password`nJWT_SECRET=$secret" | Set-Content -LiteralPath $config
}
function dc { & docker compose --env-file $config -f compose.acceptance.yaml @args; if($LASTEXITCODE -ne 0){throw 'Docker Compose command failed'} }
switch($Action){
  up { dc build; dc up -d --wait --wait-timeout 120 }
  restart { dc restart api worker web; Start-Sleep -Seconds 3; (Invoke-WebRequest http://127.0.0.1:18080/health/ready).StatusCode }
  status { dc ps }
  backup {
    $stamp=Get-Date -Format yyyyMMdd-HHmmss
    dc exec -T db pg_dump -U foodflow -d foodflow -Fc -f /tmp/acceptance.dump
    dc cp db:/tmp/acceptance.dump ".cache/acceptance-$stamp.dump"
    # Include uploaded objects independently of PostgreSQL.
    dc exec -T api tar -cf /tmp/images.tar -C /app/data images
    dc cp api:/tmp/images.tar ".cache/acceptance-images-$stamp.tar"
    Write-Output "Backup saved under .cache with timestamp $stamp"
  }
  restore-check {
    # Restore only to a fresh database; never overwrite the live acceptance database.
    $name='restore_check_'+(Get-Date -Format yyyyMMddHHmmss)
    dc exec -T db pg_dump -U foodflow -d foodflow -Fc -f /tmp/acceptance-restore.dump
    dc exec -T db createdb -U foodflow $name
    dc exec -T db pg_restore -U foodflow -d $name --exit-on-error /tmp/acceptance-restore.dump
    dc exec -T db psql -U foodflow -d $name -c 'SELECT max(version_id) AS migration FROM goose_db_version; SELECT count(*) AS users FROM users; SELECT count(*) AS ledger FROM stock_ledger;'
    Write-Output "Restored into $name; retained for inspection."
  }
}
