param([string]$Revision='',[long]$RunId=0)
$ErrorActionPreference='Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
$origin=(& git remote get-url origin).Trim()
if($LASTEXITCODE -ne 0 -or $origin -notmatch '^https://github\.com/([a-zA-Z0-9_.-]+)/([a-zA-Z0-9_.-]+?)(?:\.git)?$'){throw 'Expected an HTTPS github.com origin'}
$repo=$matches[1]+'/'+$matches[2]
# Credential Manager authorization stays in memory. Never print/store credential output.
$credentialLines="protocol=https`nhost=github.com`n`n"|& git credential fill
if($LASTEXITCODE -ne 0){throw 'GitHub Git credential authorization is unavailable'}
$credential=@{}
foreach($line in $credentialLines){$key,$value=$line -split '=',2;if($key -and $value){$credential[$key]=$value}}
if(!$credential.password){throw 'GitHub credential has no access token'}
$headers=@{Authorization='Bearer '+$credential.password;Accept='application/vnd.github+json';'X-GitHub-Api-Version'='2022-11-28'}
function API([string]$path){
  try { Invoke-RestMethod -Uri ('https://api.github.com/repos/'+$repo+$path) -Headers $headers -TimeoutSec 30 }
  catch { throw 'GitHub API request failed; verify authorization/network. Credential contents were not exported.' }
}
if($RunId){
  $run=API "/actions/runs/$RunId"
  $jobs=API "/actions/runs/$RunId/jobs"
  @{id=$run.id;revision=$run.head_sha;status=$run.status;conclusion=$run.conclusion;url=$run.html_url;jobs=@($jobs.jobs|ForEach-Object {@{name=$_.name;status=$_.status;conclusion=$_.conclusion;failed_steps=@($_.steps|Where-Object {$_.conclusion -eq 'failure'}|ForEach-Object {$_.name})}})}|ConvertTo-Json -Depth 8
}else{
  $path='/actions/workflows/ci.yml/runs?per_page=5'
  if($Revision){if($Revision -notmatch '^[a-f0-9]{40}$'){throw 'Revision must be a full SHA'};$path+='&head_sha='+$Revision}
  $runs=API $path
  @($runs.workflow_runs|ForEach-Object {@{id=$_.id;revision=$_.head_sha;status=$_.status;conclusion=$_.conclusion;url=$_.html_url}})|ConvertTo-Json -Depth 5
}
