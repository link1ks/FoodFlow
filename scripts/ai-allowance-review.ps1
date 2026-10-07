param([ValidateSet('status','acknowledge')][string]$Action='status',[string]$JobId='',[switch]$ConfirmProviderFinished)
$ErrorActionPreference='Stop'
Set-Location -LiteralPath (Split-Path $PSScriptRoot -Parent)
. (Join-Path $PSScriptRoot 'recovery-common.ps1')
if($Action -eq 'acknowledge' -and (!$ConfirmProviderFinished -or $JobId -notmatch '^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$')){throw 'Confirm provider request has ended, review its bill, and provide the exact job UUID. No automatic refunds.'}
$script:RecoveryProcessEnv=Read-RecoveryCredentials
$db=(Invoke-RecoveryDocker -Arguments @('compose','--env-file','.cache/learning.env','-f','compose.learning.yaml','ps','-q','db') -Label 'learning database').Trim()
$project=(Invoke-RecoveryDocker -Arguments @('inspect','--format','{{index .Config.Labels "com.docker.compose.project"}}',$db) -Label 'learning database identity').Trim()
if($project -ne 'foodflow-learning'){throw 'Refusing a foreign database.'}
if($Action -eq 'status'){
  $sql="SELECT state,count(*) AS calls,coalesce(sum(ceiling_milli),0) AS reserved_milli FROM ai_call_allowances GROUP BY state ORDER BY state"
  $result=Invoke-RecoveryDocker -Arguments @('exec',$db,'psql','-U','foodflow','-d','foodflow','-At','-c',$sql) -Label 'allowance summary'
  Write-Output $result
}else{
  # UUID-only input; terminal, aged jobs only. Never change the monetary debit.
  $sql="UPDATE ai_call_allowances a SET state='completed',finished_at=now() FROM jobs j WHERE a.job_id=j.id AND a.job_id='$JobId'::uuid AND a.state IN ('pending','uncertain') AND a.created_at<now()-interval '5 minutes' AND j.status IN ('failed','cancelled') AND (j.lease_until IS NULL OR j.lease_until<now()) RETURNING a.state"
  $result=Invoke-RecoveryDocker -Arguments @('exec',$db,'psql','-U','foodflow','-d','foodflow','-At','-c',$sql) -Label 'explicit provider acknowledgement'
  if($result -notmatch '(?m)^completed\r?$'){throw 'No eligible reservation acknowledged; keep it held and investigate.'}
  Write-Output 'Acknowledged completed provider request; full debit retained, no retry dispatched.'
}
