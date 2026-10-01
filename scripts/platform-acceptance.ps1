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
if(!$SkipBuild){& pwsh -NoProfile -File scripts/build-acceptance-images.ps1; if($LASTEXITCODE -ne 0){throw 'Platform image build failed'}}
dc up -d --wait --wait-timeout 300
& go run ./cmd/harness -mode images -platform-images -out .cache/harness/platform-image-binding.json
if($LASTEXITCODE -ne 0){throw 'Platform image/source binding failed; rebuild acceptance'}
& pwsh -File tests/platform.ps1
if($LASTEXITCODE -ne 0){throw 'Platform business acceptance failed'}
