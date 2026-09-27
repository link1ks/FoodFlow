# Runs only against the isolated acceptance stack, with its deterministic model.
$ErrorActionPreference='Stop'
$base='http://127.0.0.1:18080/api'
$worker='foodflow-acceptance-worker-1'
function DockerChecked { & docker @args; if($LASTEXITCODE -ne 0){throw 'Docker operation failed'} }
function Request($path,$body,$token='') {
  $headers=@{'Idempotency-Key'=[guid]::NewGuid().ToString()}
  if($token){$headers.Authorization='Bearer '+$token}
  Invoke-RestMethod -Method Post -Uri ($base+$path) -Headers $headers -ContentType application/json -Body ($body|ConvertTo-Json)
}
$suffix=[guid]::NewGuid().ToString('N')
$user=Request '/register' @{email="recovery-$suffix@example.test";password='Recovery-Test-2026!';name='Recovery test'}
$testHousehold=Request '/households' @{name='Worker recovery fixture';servings=2} $user.token
$root='/households/'+$testHousehold.id
try {
  # Disable automatic restart only for this known disposable acceptance worker.
  DockerChecked update --restart=no $worker | Out-Null
  DockerChecked kill --signal=KILL $worker | Out-Null
  $job=Request ($root+'/jobs/plan') @{day=(Get-Date).ToString('yyyy-MM-dd');meal='dinner';servings=2;max_minutes=30} $user.token
  $headers=@{Authorization='Bearer '+$user.token}
  $queued=Invoke-RestMethod -Uri ($base+$root+'/jobs/'+$job.id) -Headers $headers
  if($queued.status -ne 'queued'){throw 'Job not durably queued while worker is down'}
  DockerChecked start $worker | Out-Null
  for($i=0;$i -lt 60;$i++){
    Start-Sleep -Milliseconds 500
    $result=Invoke-RestMethod -Uri ($base+$root+'/jobs/'+$job.id) -Headers $headers
    if($result.status -eq 'awaiting_confirmation'){break}
    if($result.status -eq 'failed'){throw 'Recovered job failed'}
  }
  if($result.status -ne 'awaiting_confirmation'){throw 'Recovery deadline exceeded'}
  $plan=Invoke-RestMethod -Uri ($base+$root+'/plans/'+$result.result.plan_id) -Headers $headers
  if($plan.status -ne 'draft'){throw 'Recovery wrote an unconfirmed business result'}
  Write-Output 'PASS: SIGKILL worker, durable enqueue during downtime, restart and draft recovery; no automatic confirmation'
} finally {
  DockerChecked update --restart=unless-stopped $worker | Out-Null
  DockerChecked start $worker | Out-Null
}
