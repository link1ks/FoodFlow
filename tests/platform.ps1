$ErrorActionPreference='Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
$base='http://127.0.0.1:18080/api'
$config=Join-Path $PWD '.cache/acceptance.env'
function dc { & docker compose --env-file $config -f compose.acceptance.yaml -f compose.platform.yaml @args; if($LASTEXITCODE -ne 0){throw 'Platform compose failed'} }
function Call($method,$path,$body=$null,$token='',$key='') {
 $headers=@{};if($token){$headers.Authorization='Bearer '+$token};if($method -ne 'GET'){$headers['Idempotency-Key']=if($key){$key}else{[guid]::NewGuid().ToString()}}
 $args=@{Method=$method;Uri=$base+$path;Headers=$headers;ContentType='application/json'}
 if($null -ne $body){$args.Body=$body|ConvertTo-Json -Compress};Invoke-RestMethod @args
}
function Check($ok,$message){if(!$ok){throw $message}}
function WaitSummary($predicate){
 for($i=0;$i -lt 90;$i++){$value=Call GET ($root+'/insights') $null $user.token;if(&$predicate $value){return $value};Start-Sleep -Milliseconds 500}
 throw 'Projection did not converge before deadline'
}
$suffix=[guid]::NewGuid().ToString('N')
$user=Call POST /register @{email="platform-$suffix@example.test";password='Platform-Test-2026!';name='Platform test'}
$household=Call POST /households @{name='Event pipeline acceptance';servings=2} $user.token
$root='/households/'+$household.id
$catalog=Call GET /ingredient-catalog $null $user.token
$tomato=$catalog|Where-Object name -EQ '番茄'|Select-Object -First 1
Check ($null -ne $tomato) 'Catalog missing tomato'
$headers=@{Authorization='Bearer '+$user.token}
$cached=Invoke-WebRequest -Uri ($base+'/ingredient-catalog') -Headers $headers
Check ($cached.Headers['X-Catalog-Cache'] -eq 'hit') 'Redis did not serve catalog'
try{Invoke-WebRequest -Uri ($base+'/ingredient-catalog')|Out-Null;throw 'Unauthenticated cached catalog access succeeded'}catch{if(!$_.Exception.Response -or [int]$_.Exception.Response.StatusCode -ne 401){throw}}
$key=[guid]::NewGuid().ToString()
$body=@{catalog_id=$tomato.id;quantity='100'}
$stock=Call POST ($root+'/catalog-stock') $body $user.token $key
Call POST ($root+'/catalog-stock') $body $user.token $key|Out-Null
$first=WaitSummary {param($s) $s.items.Count -eq 1 -and $s.items[0].inbound_milli -eq '100000'}
Check ($first.items[0].unit -eq 'g') 'Unit corrupted'
try{
 dc stop redis | Out-Null
 $fallback=Invoke-WebRequest -Uri ($base+'/ingredient-catalog') -Headers $headers
 Check ($fallback.StatusCode -eq 200 -and $fallback.Headers['X-Catalog-Cache'] -eq 'database') 'Redis outage did not fall back'
 dc stop kafka | Out-Null
 Call POST ($root+'/stock') @{batch_id=$stock.batch_id;ingredient_id=$stock.ingredient_id;quantity='10';reason='consume'} $user.token|Out-Null
 $inventory=Call GET ($root+'/inventory') $null $user.token
 Check ($inventory.items[0].quantity -eq '90.000') 'Broker outage affected authoritative inventory'
}finally{dc start redis kafka | Out-Null}
$after=WaitSummary {param($s) $s.items[0].consumed_milli -eq '10000'}
try{
 dc stop insights | Out-Null
 Call POST ($root+'/stock') @{batch_id=$stock.batch_id;ingredient_id=$stock.ingredient_id;quantity='5';reason='waste'} $user.token|Out-Null
}finally{dc start insights | Out-Null}
$result=WaitSummary {param($s) $s.items[0].wasted_milli -eq '5000'}
$raw=dc exec -T db psql -U foodflow -d foodflow -At -c "SELECT payload::text FROM stock_outbox WHERE household_id='$($household.id)' ORDER BY created_at LIMIT 1"
$beforeRejected=dc exec -T insights-db psql -U insights -d insights -At -c 'SELECT count(*) FROM rejected_events'
@(($raw -join "`n"),'{"version":99}') | & docker compose --env-file $config -f compose.acceptance.yaml -f compose.platform.yaml exec -T kafka /opt/kafka/bin/kafka-console-producer.sh --bootstrap-server kafka:9092 --topic foodflow.stock.v1
if($LASTEXITCODE -ne 0){throw 'Duplicate event injection failed'}
for($i=0;$i -lt 30;$i++){
 $rejected=dc exec -T insights-db psql -U insights -d insights -At -c 'SELECT count(*) FROM rejected_events'
 if([int]$rejected -gt [int]$beforeRejected){break};Start-Sleep -Milliseconds 500
}
Check ([int]$rejected -gt [int]$beforeRejected) 'Malformed Kafka event was not quarantined'
$count=dc exec -T insights-db psql -U insights -d insights -At -c "SELECT count(*) FROM stock_facts WHERE household_id='$($household.id)'"
Check ($count.Trim() -eq '3') 'Repeated Kafka delivery counted more than once'
$other=Call POST /register @{email="outsider-$suffix@example.test";password='Platform-Test-2026!';name='Outsider'}
try{Call GET ($root+'/insights') $null $other.token|Out-Null;throw 'Cross-household projection access succeeded'}catch{if(!$_.Exception.Response -or [int]$_.Exception.Response.StatusCode -ne 404){throw}}
$evidence=@{recorded_at=[DateTimeOffset]::UtcNow.ToString('o');passed=$true;scenarios=@('redis-hit','cached-auth','redis-fallback','broker-outage-stock-write','consumer-restart','duplicate-delivery','invalid-event-quarantine','household-isolation');projection_fact_count=3}
$evidence|ConvertTo-Json -Depth 3|Set-Content -LiteralPath .cache/platform-evidence.json
Write-Output 'PASS: Redis hit/auth/fallback, atomic Kafka outage stock write, consumer restart, duplicate delivery, invalid event quarantine and household isolation'
