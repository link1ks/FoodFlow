# Exercise a replacement API with a distinct IP, without changing the web container.
$ErrorActionPreference='Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
$config=Join-Path $PWD '.cache/acceptance.env'
function dc { & docker compose --env-file $config -f compose.acceptance.yaml -f compose.platform.yaml @args; if($LASTEXITCODE -ne 0){throw 'Acceptance compose failed'} }
function WaitGateway {
  for($attempt=0;$attempt -lt 40;$attempt++){
    try{
      $ready=Invoke-WebRequest http://127.0.0.1:15173/health/ready -SkipHttpErrorCheck -TimeoutSec 2
      if($ready.StatusCode -eq 200){return}
    }catch{}
    Start-Sleep -Milliseconds 500
  }
  throw 'Gateway did not resolve the currently active API before deadline'
}
$webId=dc ps -q web
$apiId=dc ps -q api
$before=& docker inspect --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $apiId
$network=& docker inspect --format '{{range .NetworkSettings.Networks}}{{.NetworkID}}{{end}}' $apiId
$replacement='ff-gateway-api-'+[guid]::NewGuid().ToString('N').Substring(0,8)
$disconnected=$false
try {
  # Original keeps its IP while the replacement is allocated. Compose supplies
  # env/volumes without exporting secrets; one-off run publishes no host port.
  dc run -d --no-deps --use-aliases --name $replacement api | Out-Null
  $after=& docker inspect --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $replacement
  if($LASTEXITCODE -ne 0 -or !$after -or $after -eq $before){throw 'Replacement must have a distinct API address'}
  & docker network disconnect $network $apiId
  if($LASTEXITCODE -ne 0){throw 'Cannot detach original API during isolated cutover'}
  $disconnected=$true
  dc stop api | Out-Null
  WaitGateway
  $login=Invoke-WebRequest -Method Post -Uri http://127.0.0.1:15173/api/login -ContentType application/json -Body '{"account":"missing-user@example.test","password":"invalid-test-password"}' -SkipHttpErrorCheck -TimeoutSec 5
  if($login.StatusCode -ne 401 -or $login.Headers['Content-Type'] -notlike 'application/json*'){throw 'Replacement login returned a gateway error instead of credential validation'}
  $null=$login.Content|ConvertFrom-Json
  if((dc ps -q web) -ne $webId){throw 'Test accidentally replaced the web gateway'}
  Write-Output "PASS: unchanged web routes health/login to replacement API (IP $before -> $after)"
} finally {
  & docker rm -f $replacement 2>$null | Out-Null
  if($disconnected){
    & docker network connect --alias api $network $apiId
    if($LASTEXITCODE -ne 0){throw 'Cannot restore original API network'}
  }
  dc up -d --no-deps --wait api | Out-Null
  WaitGateway
}
