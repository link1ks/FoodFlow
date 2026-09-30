param([switch]$SkipBuild)
$ErrorActionPreference='Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
$config=Join-Path $PWD '.cache/acceptance.env'
if(!(Test-Path -LiteralPath $config)){throw 'Run scripts/local-acceptance.ps1 up first'}
foreach($name in @('INSIGHTS_SERVICE_TOKEN','INSIGHTS_DB_PASSWORD')){
 if(!(Select-String -LiteralPath $config -Pattern "^$name=" -Quiet)){
  $value=[Convert]::ToHexString([Security.Cryptography.RandomNumberGenerator]::GetBytes(32))
  Add-Content -LiteralPath $config -Value "$name=$value"
 }
}
function dc { & docker compose --env-file $config -f compose.acceptance.yaml -f compose.platform.yaml @args; if($LASTEXITCODE -ne 0){throw 'Platform compose failed'} }
if(!$SkipBuild){dc build}
dc up -d --wait --wait-timeout 300
& pwsh -File tests/platform.ps1
if($LASTEXITCODE -ne 0){throw 'Platform business acceptance failed'}
