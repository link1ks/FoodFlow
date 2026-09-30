# Regression for Nginx retaining an old upstream IP after API replacement.
$ErrorActionPreference='Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
$config=Join-Path $PWD '.cache/acceptance.env'
function dc { & docker compose --env-file $config -f compose.acceptance.yaml -f compose.platform.yaml @args; if($LASTEXITCODE -ne 0){throw 'Acceptance compose failed'} }
$webId=dc ps -q web
$apiId=dc ps -q api
$before=& docker inspect --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $apiId
$network=& docker inspect --format '{{range .NetworkSettings.Networks}}{{.NetworkID}}{{end}}' $apiId
$reservation='ff-gateway-ip-'+[guid]::NewGuid().ToString('N').Substring(0,8)
try {
dc stop api
dc rm -f api
# Occupy the previous address briefly so replacement must receive a new IP.
& docker run -d --name $reservation --network $network --ip $before alpine:3.21 sleep 120 | Out-Null
if($LASTEXITCODE -ne 0){throw 'Cannot reserve old API address'}
dc up -d --no-deps api
$after=& docker inspect --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' (dc ps -q api)
if($after -eq $before){throw 'API address did not change; DNS regression was not exercised'}
for($i=0;$i -lt 40;$i++){
 try{$ready=Invoke-WebRequest http://127.0.0.1:15173/health/ready -SkipHttpErrorCheck;if($ready.StatusCode -eq 200){break}}catch{}
 Start-Sleep -Milliseconds 500
}
if(!$ready -or $ready.StatusCode -ne 200){throw 'Gateway failed to resolve replacement API'}
$login=Invoke-WebRequest -Method Post -Uri http://127.0.0.1:15173/api/login -ContentType application/json -Body '{"account":"missing-user@example.test","password":"invalid-test-password"}' -SkipHttpErrorCheck
if($login.StatusCode -ne 401 -or $login.Headers['Content-Type'] -notlike 'application/json*'){throw 'Login returned a gateway error instead of credential validation'}
$null=$login.Content|ConvertFrom-Json
if((dc ps -q web) -ne $webId){throw 'Test accidentally replaced the web gateway'}
Write-Output "PASS: unchanged web gateway routes health and login after API replacement (IP $before -> $after)"
} finally {
 & docker rm -f $reservation | Out-Null
 dc up -d --no-deps api
}
