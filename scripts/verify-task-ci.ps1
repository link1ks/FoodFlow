$ErrorActionPreference='Stop'
Set-Location -LiteralPath (Split-Path $PSScriptRoot -Parent)
$revision=(& git rev-parse HEAD).Trim()
if($LASTEXITCODE -ne 0){throw 'Cannot read current commit'}
$status=& git status --porcelain
if($LASTEXITCODE -ne 0 -or $status){throw 'Cloud completion requires clean committed source'}
$remote=(& git ls-remote origin refs/heads/main)
if($LASTEXITCODE -ne 0 -or !$remote -or ($remote -split '\s+')[0] -ne $revision){throw 'Current commit is not published on origin/main'}
$json=& pwsh -NoProfile -File scripts/ci-status.ps1 -Revision $revision
if($LASTEXITCODE -ne 0){throw 'Cannot read commit CI'}
$runs=@(($json -join "`n") | ConvertFrom-Json)
$run=$runs | Where-Object { $_.revision -eq $revision -and $_.status -eq 'completed' -and $_.conclusion -eq 'success' } | Select-Object -First 1
if(!$run){throw 'Current commit has no completed successful CI run'}
$detail=& pwsh -NoProfile -File scripts/ci-status.ps1 -RunId $run.id
if($LASTEXITCODE -ne 0){throw 'Cannot read CI jobs'}
$result=($detail -join "`n") | ConvertFrom-Json
foreach($name in @('backend','frontend','images','platform')){
  $job=@($result.jobs | Where-Object name -eq $name)
  if($job.Count -ne 1 -or $job[0].conclusion -ne 'success'){throw "Missing successful CI job: $name"}
}
@{revision=$revision;run_id=$run.id;url=$run.url;status='passed'} | ConvertTo-Json -Compress
