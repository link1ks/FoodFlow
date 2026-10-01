$ErrorActionPreference='Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
$config=Join-Path $PWD '.cache/acceptance.env'
if(!(Test-Path -LiteralPath $config)){throw 'Isolated acceptance configuration is missing'}
$dbId=(& docker compose -p foodflow-acceptance --env-file $config -f compose.acceptance.yaml ps -q db)
if($LASTEXITCODE -ne 0 -or !$dbId){throw 'Isolated acceptance database is unavailable'}
$dbId=$dbId.Trim()
$labels=(& docker inspect --format '{{json .Config.Labels}}' $dbId) | ConvertFrom-Json
if($LASTEXITCODE -ne 0 -or $labels.'com.docker.compose.project' -ne 'foodflow-acceptance' -or $labels.'com.docker.compose.service' -ne 'db'){
  throw 'Refusing to prepare a database outside the isolated acceptance project'
}
# Repeated synthetic signups can exhaust the shared browser proxy IP budget.
# Advance only the fixture's rate-limit clock; the next request still executes
# normal application reset/increment logic. Never touch development records,
# sessions, users, stock, ledger or volumes. No production limit is disabled.
& docker exec $dbId psql -U foodflow -d foodflow -v ON_ERROR_STOP=1 -q -c "UPDATE auth_rate_limits SET window_start=now()-interval '16 minutes' WHERE window_start>=now()-interval '15 minutes';"
if($LASTEXITCODE -ne 0){throw 'Acceptance auth clock preparation failed'}
Write-Output 'PASS: isolated browser auth budget window expired; application limits remain enabled'
