param([ValidateSet('prepare','check','switch')][string]$Action='check',[string]$Release='',
  [string]$Full='.cache/harness/learning-release-full.json',
  [string]$Acceptance='.cache/harness/learning-release-acceptance.json',[switch]$ConfirmSwitch)
$ErrorActionPreference='Stop'
Set-Location -LiteralPath (Split-Path $PSScriptRoot -Parent)
. (Join-Path $PSScriptRoot 'recovery-common.ps1')
. (Join-Path $PSScriptRoot 'release-common.ps1')
function ReleaseDocker([string[]]$Arguments,[string]$Label){Invoke-RecoveryDocker -Arguments $Arguments -Label $Label}
function ReleaseInspect([string]$ID,[string]$Template){(ReleaseDocker @('inspect','--format',$Template,$ID) 'release identity').Trim()}
function ReleaseContainers {
  $ids=@{}
  foreach($service in @('db','api','worker','web')){
    $id=(ReleaseDocker @('ps','-q','--no-trunc','--filter','label=com.docker.compose.project=foodflow-learning','--filter',"label=com.docker.compose.service=$service") 'learning identity').Trim()
    if($id -notmatch '^[a-f0-9]{64}$' -or (ReleaseInspect $id '{{.State.Running}}') -ne 'true'){throw 'Expected exactly one running learning service.'}
    $ids[$service]=$id
  }
  return $ids
}
function HistoryDigest([string]$DB){
  $history=ReleaseDocker @('exec',$DB,'psql','-U','foodflow','-d','foodflow','-At','-c','SELECT version_id,is_applied FROM goose_db_version ORDER BY id') 'migration history'
  return [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes($history.Replace("`r`n","`n")))).ToLowerInvariant()
}
function ValidateRuntime($Manifest,$Containers){
  $compose=(Get-RecoveryArtifact (Join-Path $PWD 'compose.learning.yaml')).sha256
  Assert-ReleaseCompatible $Manifest (Get-ReleaseSchemaDigest $PWD) (HistoryDigest $Containers.db) $compose (ReleaseInspect $Containers.db '{{.Image}}')
  foreach($service in @('api','web')){
    $id=(ReleaseDocker @('image','inspect','--format','{{.Id}}',$Manifest.images[$service]) 'retained image').Trim()
    $bound=(ReleaseDocker @('image','inspect','--format','{{index .Config.Labels "io.foodflow.source-digest"}}',$id) 'retained source binding').Trim()
    if($id -cne $Manifest.images[$service] -or $bound -cne $Manifest.source_digest){throw 'Retained release image is missing or does not match its source.'}
  }
}
$lock=$null
try {
  if($Action -eq 'switch' -and !$ConfirmSwitch){throw 'Switch requires explicit -ConfirmSwitch; check is read-only.'}
  if($Action -eq 'prepare'){
    if($Release){throw 'Release destination is generated automatically.'}
    $clean=(& git status --porcelain) -join "`n"
    if($LASTEXITCODE -ne 0 -or $clean){throw 'Commit or resolve worktree changes before preparing a release.'}
    $revision=(& git rev-parse HEAD).Trim()
    if($LASTEXITCODE -ne 0 -or $revision -notmatch '^[a-f0-9]{40}$'){throw 'Cannot determine release revision.'}
    $digest=(& go run ./cmd/harness -mode fingerprint).Trim()
    if($LASTEXITCODE -ne 0){throw 'Cannot determine source digest.'}
    Assert-ReleaseVerified (Get-Content -LiteralPath $Full -Raw | ConvertFrom-Json) (Get-Content -LiteralPath $Acceptance -Raw | ConvertFrom-Json) $digest
  }else{
    if(!$Release){throw 'Provide the exact release folder with -Release.'}
    # Check public artifacts before credentials, Docker or backup side effects.
    $manifest=Assert-ReleaseBundle $Release
  }
  $script:RecoveryProcessEnv=Read-RecoveryCredentials
  $containers=ReleaseContainers
  if($Action -eq 'prepare'){
    $root=Join-Path $PWD '.cache/learning-releases';New-RecoveryDirectory $root
    $directory=Join-Path $root ('release-'+[DateTime]::UtcNow.ToString('yyyyMMddTHHmmssZ')+'-'+$revision.Substring(0,8)+'-'+[guid]::NewGuid().ToString('N').Substring(0,6));New-RecoveryDirectory $directory
    $manifest=@{version=1;project='foodflow-learning';revision=$revision;created_at=[DateTime]::UtcNow.ToString('o');source_digest=$digest;schema_digest=(Get-ReleaseSchemaDigest $PWD);history_digest=(HistoryDigest $containers.db);images=@{api=(ReleaseInspect $containers.api '{{.Image}}');web=(ReleaseInspect $containers.web '{{.Image}}');db=(ReleaseInspect $containers.db '{{.Image}}')};verification=@{full=(Get-Content $Full -Raw|ConvertFrom-Json).run_id;acceptance=(Get-Content $Acceptance -Raw|ConvertFrom-Json).run_id};artifacts=@{}}
    if((ReleaseInspect $containers.worker '{{.Image}}') -cne $manifest.images.api){throw 'API and Worker image identities differ.'}
    Write-ReleaseSourceArchive $directory $revision
    Copy-Item -LiteralPath compose.learning.yaml -Destination (Join-Path $directory 'compose.learning.yaml')
    foreach($name in @('source.zip','compose.learning.yaml')){$manifest.artifacts[$name]=Get-RecoveryArtifact (Join-Path $directory $name)}
    ValidateRuntime $manifest $containers
    Assert-ReleaseSourceArchive (Join-Path $directory 'source.zip')
    $afterDigest=(& go run ./cmd/harness -mode fingerprint).Trim()
    if($LASTEXITCODE -ne 0 -or $afterDigest -cne $digest -or ((& git status --porcelain) -join "`n") -or ((& git rev-parse HEAD).Trim()) -cne $revision){throw 'Source changed during release preparation; incomplete bundle retained.'}
    Write-RecoveryPrivateJSON (Join-Path $directory 'manifest.json') $manifest
    $null=Assert-ReleaseBundle $directory
    Write-Output "PASS: committed source and bound runtime release saved to $directory (images retained locally; no credentials or data)."
  }else{
    ValidateRuntime $manifest $containers
    if($Action -eq 'switch'){
      # Paired backup resumes writers itself; switch never performs a database downgrade.
      & pwsh -NoProfile -File scripts/learning-recovery.ps1 backup
      if($LASTEXITCODE -ne 0){throw 'Pre-switch backup failed; no release switch performed.'}
      $lock=[IO.File]::Open((Join-Path $PWD '.cache/learning-recovery.lock'),[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None)
      $containers=ReleaseContainers;ValidateRuntime $manifest $containers
      $active=(ReleaseDocker @('exec',$containers.db,'psql','-U','foodflow','-d','foodflow','-At','-c',"SELECT count(*) FROM jobs WHERE status IN ('queued','running')") 'active jobs').Trim()
      if($active -ne '0'){throw 'Wait for queued/running jobs before switching versions.'}
      $overlay=Join-Path $PWD ('.cache/release-switch-'+[guid]::NewGuid().ToString('N')+'.json')
      Write-RecoveryPrivateJSON $overlay @{services=@{api=@{image=$manifest.images.api};worker=@{image=$manifest.images.api};web=@{image=$manifest.images.web}}}
      $null=ReleaseDocker @('image','tag',$manifest.images.api,'foodflow-api:learning') 'API tag'
      $null=ReleaseDocker @('image','tag',$manifest.images.web,'foodflow-web:learning') 'web tag'
      $null=ReleaseDocker @('compose','--env-file','.cache/learning.env','-f','compose.learning.yaml','-f',$overlay,'up','-d','--no-build','--no-deps','--force-recreate','--wait','--wait-timeout','120','api','worker','web') 'same-schema version switch'
      $after=ReleaseContainers
      foreach($service in @('api','worker','web')){if((ReleaseInspect $after[$service] '{{.Image}}') -cne $manifest.images[$(if($service -eq 'worker'){'api'}else{$service})]){throw 'Switched runtime image differs from release.'}}
      if($after.db -cne $containers.db -or (HistoryDigest $after.db) -cne $manifest.history_digest){throw 'Database changed unexpectedly during switch.'}
      if((Invoke-WebRequest http://127.0.0.1:17173/health/ready -TimeoutSec 5).StatusCode -ne 200){throw 'Switched gateway is not ready.'}
      Write-Output 'PASS: same-schema application switch ready; database container, volumes and migration history retained.'
    }else{Write-Output 'PASS: source artifact integrity, retained image bindings, current schema/history/topology verified; no services changed.'}
  }
}finally{
  if($lock){$lock.Dispose();Remove-Item -LiteralPath (Join-Path $PWD '.cache/learning-recovery.lock')}
}
