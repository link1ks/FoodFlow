param([string]$Base='http://localhost:8080')
$ErrorActionPreference='Stop'
$suffix=[guid]::NewGuid().ToString('N').Substring(0,8)
function Call($method,$path,$body=$null,$token='',$key='') {
  if($method -notin @('GET','HEAD') -and !$key){$key=[guid]::NewGuid().ToString()}
  $headers=@{}; if($token){$headers['Authorization']='Bearer '+$token}; if($key){$headers['Idempotency-Key']=$key}
  $args=@{Method=$method;Uri=$Base+'/api'+$path;Headers=$headers;ContentType='application/json'}
  if($null -ne $body){$args.Body=($body|ConvertTo-Json -Depth 15 -Compress)}
  return Invoke-RestMethod @args
}
function Check($ok,$message){if(-not $ok){throw $message}}
$u=Call 'POST' '/register' @{email="a$suffix@example.com";password='password123';name='Alice'}
$h=Call 'POST' '/households' @{name='My home';servings=2;preferences=''} $u.token
$root='/households/'+$h.id
$v=Call 'POST' '/register' @{email="b$suffix@example.com";password='password123';name='Bob'}
$other=Call 'POST' '/households' @{name='Other home';servings=2;preferences=''} $v.token
try{Call 'GET' ($root+'/inventory') $null $v.token|Out-Null;throw 'cross household access succeeded'}catch{if(!$_.Exception.Response -or [int]$_.Exception.Response.StatusCode -ne 404){throw}}
$invite=Call 'POST' ($root+'/invites') @{email="b$suffix@example.com";role='editor'} $u.token
Call 'POST' '/invites/accept' @{code=$invite.code} $v.token|Out-Null
$members=Call 'GET' ($root+'/members') $null $v.token
Check ($members.Count -eq 2) 'invite acceptance failed'
$ingredient=Call 'POST' ($root+'/ingredients') @{name='生菜';category='蔬菜';unit='g';low='100'} $u.token
$stockKey=[guid]::NewGuid().ToString()
$stockBody=@{ingredient_id=$ingredient.id;quantity='300';reason='manual';source='smoke'}
$batch=Call 'POST' ($root+'/stock') $stockBody $u.token $stockKey
$again=Call 'POST' ($root+'/stock') $stockBody $u.token $stockKey
Check ($again.batch_id -eq $batch.batch_id) 'idempotency replay changed result'
$inventory=Call 'GET' ($root+'/inventory') $null $u.token
Check ($inventory.items[0].quantity -eq '300.000') 'duplicate stock write'
$job=Call 'POST' ($root+'/jobs/plan') @{day=(Get-Date).ToString('yyyy-MM-dd');meal='dinner';servings=2;max_minutes=15;preference='清淡'} $u.token
$detail=$null
for($i=0;$i -lt 30;$i++){Start-Sleep -Milliseconds 500;$detail=Call 'GET' ($root+'/jobs/'+$job.id) $null $u.token;if($detail.status -eq 'awaiting_confirmation'){break}}
Check ($detail.status -eq 'awaiting_confirmation') ('worker status '+$detail.status+' '+$detail.error)
$plan=Call 'GET' ($root+'/plans/'+$detail.result.plan_id) $null $u.token
Check ($plan.status -eq 'draft') 'plan not draft'
Call 'POST' ($root+'/plans/'+$plan.id+'/confirm') @{revision=$plan.revision;accept_uncertain=$false} $u.token|Out-Null
$items=Call 'GET' ($root+'/shopping') $null $v.token
foreach($item in $items){Call 'PATCH' ($root+'/shopping/'+$item.id) @{checked=$true;bought=$item.needed} $v.token|Out-Null;Call 'POST' ($root+'/shopping/'+$item.id+'/stock') @{location='fridge';source='smoke';bought_on=(Get-Date).ToString('yyyy-MM-dd')} $v.token ([guid]::NewGuid().ToString())|Out-Null}
Call 'POST' ($root+'/plans/'+$plan.id+'/consume') @{} $u.token ([guid]::NewGuid().ToString())|Out-Null
$final=Call 'GET' ($root+'/plans/'+$plan.id) $null $u.token
Check ($final.status -eq 'consumed') 'meal consumption failed'
Write-Output "PASS registration, household isolation, invite, idempotent stock, worker plan, shopping stock, meal consumption. Plan=$($plan.id)"
