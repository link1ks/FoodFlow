param([ValidateSet('backup','verify','restore-check')][string]$Action='verify',[string]$Backup='')
$ErrorActionPreference='Stop'
Set-Location -LiteralPath (Split-Path $PSScriptRoot -Parent)
. (Join-Path $PSScriptRoot 'recovery-common.ps1')
$script:RecoveryProcessEnv=Read-RecoveryCredentials
$sourceArgs=@('compose','--env-file','.cache/learning.env','-f','compose.learning.yaml')
function SourceDocker([string[]]$Tail,[string]$Label){Invoke-RecoveryDocker -Arguments ($sourceArgs+$Tail) -Label $Label}
function Inspect([string]$ID,[string]$Template){(Invoke-RecoveryDocker -Arguments @('inspect','--format',$Template,$ID) -Label 'identity').Trim()}
function Snapshot {
  $raw=SourceDocker @('run','--rm','--no-deps','-T','api','/app/recovery','-mode','snapshot') 'read-only snapshot'
  return ($raw|ConvertFrom-Json -AsHashtable)
}
function SameSnapshot($A,$B){return (($A|ConvertTo-Json -Depth 20 -Compress) -eq ($B|ConvertTo-Json -Depth 20 -Compress))}
function SourceReady {
  for($i=0;$i -lt 30;$i++){
    try {if((Invoke-WebRequest http://127.0.0.1:17173/health/ready -TimeoutSec 2).StatusCode -eq 200){return}}catch{}
    Start-Sleep -Milliseconds 500
  }
  throw 'Original learning gateway did not become ready after backup.'
}
function ValidateArchive($Manifest,[string]$Directory){
  $args=@('run','--rm','-i','--network','none','--user','10001:10001','--read-only','--cap-drop','ALL','--security-opt','no-new-privileges:true',$Manifest.api_image,'/app/recovery','-mode','inspect','-files',[string]$Manifest.snapshot.images.files,'-bytes',[string]$Manifest.snapshot.images.bytes)
  $actual=Invoke-RecoveryDocker -Arguments $args -Label 'image archive validation' -InputPath (Join-Path $Directory 'images.tar')
  $images=$actual|ConvertFrom-Json -AsHashtable
  foreach($key in @('files','bytes','sha256')){if($images[$key] -ne $Manifest.snapshot.images[$key]){throw 'Image archive does not match snapshot.'}}
}
New-Item -ItemType Directory -Force .cache/harness | Out-Null
$lockPath=Join-Path $PWD '.cache/learning-recovery.lock'
$lock=$null;$restart=@();$restoreArgs=$null
$timer=[Diagnostics.Stopwatch]::StartNew()
try {
  $lock=[IO.File]::Open($lockPath,[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None)
  if($Action -eq 'backup'){
    if($Backup){throw 'Backup destination is generated automatically.'}
    $root=Join-Path $PWD '.cache/learning-backups';New-RecoveryDirectory $root
    $id='backup-'+[DateTime]::UtcNow.ToString('yyyyMMddTHHmmssZ')+'-'+[guid]::NewGuid().ToString('N').Substring(0,8)
    $directory=Join-Path $root $id;New-RecoveryDirectory $directory
    $containers=@{}
    foreach($service in @('db','api','worker')){
      $container=(SourceDocker @('ps','-a','-q',$service) 'source container').Trim()
      if($container -notmatch '^[a-f0-9]{64}$' -or (Inspect $container '{{index .Config.Labels "com.docker.compose.project"}}') -ne 'foodflow-learning' -or (Inspect $container '{{index .Config.Labels "com.docker.compose.service"}}') -ne $service){throw 'Refusing a missing or foreign source service.'}
      $containers[$service]=$container
    }
    if((Inspect $containers.db '{{.State.Running}}') -ne 'true'){throw 'Source database is not running.'}
    $apiImage=Inspect $containers.api '{{.Image}}';$dbImage=Inspect $containers.db '{{.Image}}'
    $tag=(Invoke-RecoveryDocker -Arguments @('image','inspect','--format','{{.Id}}','foodflow-api:learning') -Label 'source image').Trim()
    if($tag -ne $apiImage){throw 'Recreate learning deployment before backup; API image tag drifted.'}
    foreach($service in @('api','worker')){if((Inspect $containers[$service] '{{.State.Running}}') -eq 'true'){$restart+=$containers[$service]}}
    $pause=[Diagnostics.Stopwatch]::StartNew()
    if($restart.Count){$null=Invoke-RecoveryDocker -Arguments (@('stop','--time','15')+$restart) -Label 'pause kitchen writers'}
    $before=Snapshot
    if($before.postgres_major -ne 16){throw 'This recovery format requires PostgreSQL 16.'}
    $dump=Join-Path $directory 'database.dump';$archive=Join-Path $directory 'images.tar'
    Invoke-RecoveryDocker -Arguments @('exec',$containers.db,'pg_dump','-U','foodflow','-d','foodflow','-Fc','--no-owner','--no-privileges') -Label 'PostgreSQL dump' -OutputPath ($dump+'.partial')
    [IO.File]::Move($dump+'.partial',$dump)
    Invoke-RecoveryDocker -Arguments ($sourceArgs+@('run','--rm','--no-deps','-T','api','/app/recovery','-mode','archive')) -Label 'photo archive' -OutputPath ($archive+'.partial')
    [IO.File]::Move($archive+'.partial',$archive)
    $after=Snapshot
    if(!(SameSnapshot $before $after)){throw 'Source changed during backup; incomplete bundle retained.'}
    $manifest=@{version=1;id=$id;created_at=[DateTime]::UtcNow.ToString('o');source_project='foodflow-learning';api_image=$apiImage;db_image=$dbImage;snapshot=$before;artifacts=@{'database.dump'=(Get-RecoveryArtifact $dump);'images.tar'=(Get-RecoveryArtifact $archive)}}
    ValidateArchive $manifest $directory
    Write-RecoveryPrivateJSON (Join-Path $directory 'manifest.json') $manifest
    if($restart.Count){$null=Invoke-RecoveryDocker -Arguments (@('start')+$restart) -Label 'resume kitchen writers';$restart=@();SourceReady}
    $pause.Stop()
    @{passed=$true;backup=$directory;recorded_at=[DateTime]::UtcNow.ToString('o');tables=$before.tables.Count;image_files=$before.images.files;referenced_images=$before.referenced_images;writer_pause_seconds=[Math]::Round($pause.Elapsed.TotalSeconds,3);checks=@('writers quiesced','read-only snapshots stable','database and photos checksummed','photo references exist','source services resumed');limitations=@('private same-machine backup; no offsite copy or encryption','restore drill required separately')}|ConvertTo-Json -Depth 5|Set-Content -LiteralPath .cache/harness/learning-backup.json -Encoding utf8NoBOM
    Write-Output "PASS: complete private backup saved to $directory"
  }else{
    if(!$Backup){throw 'Provide the exact backup directory with -Backup.'}
    $directory=(Get-Item -LiteralPath $Backup).FullName
    # Check both artifacts before creating any recovery target.
    $manifest=Assert-RecoveryBundle $directory
    ValidateArchive $manifest $directory
    if($Action -eq 'verify'){Write-Output 'PASS: database and image artifact hashes, lengths and archive tree match.'}
    else {
      $project='foodflow-restore-'+[guid]::NewGuid().ToString('N')
      foreach($kind in @('container','volume','network')){
        $existing=Invoke-RecoveryDocker -Arguments @($kind,'ls','-q','--filter',"label=com.docker.compose.project=$project") -Label 'fresh target guard'
        if($existing.Trim()){throw 'Recovery project already exists; refusing reuse.'}
      }
      $restoreRoot=Join-Path $PWD '.cache/learning-restores';New-RecoveryDirectory $restoreRoot
      $restoreEnv=@{RECOVERY_PROJECT=$project;RECOVERY_API_IMAGE=$manifest.api_image;RECOVERY_DB_IMAGE=$manifest.db_image;RECOVERY_POSTGRES_PASSWORD=[Convert]::ToHexString([Security.Cryptography.RandomNumberGenerator]::GetBytes(24));RECOVERY_JWT_SECRET=[Convert]::ToHexString([Security.Cryptography.RandomNumberGenerator]::GetBytes(48))}
      $config=Join-Path $restoreRoot ($project+'.env')
      $stream=[IO.File]::Open($config,[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None);$stream.Dispose();Set-RecoveryPrivatePath $config
      [IO.File]::WriteAllText($config,(($restoreEnv.Keys|Sort-Object|ForEach-Object {$_+'='+$restoreEnv[$_]}) -join "`n")+"`n")
      foreach($key in $restoreEnv.Keys){$script:RecoveryProcessEnv[$key]=$restoreEnv[$key]}
      $restoreArgs=@('compose','--env-file',$config,'-f','compose.restore.yaml')
      $null=Invoke-RecoveryDocker -Arguments ($restoreArgs+@('up','-d','--wait','--wait-timeout','90','db')) -Label 'fresh recovery target'
      $db=(Invoke-RecoveryDocker -Arguments ($restoreArgs+@('ps','-q','db')) -Label 'restore DB identity').Trim()
      if((Inspect $db '{{index .Config.Labels "com.docker.compose.project"}}') -ne $project){throw 'Foreign recovery database.'}
      $count=(Invoke-RecoveryDocker -Arguments @('exec',$db,'psql','-U','foodflow','-d','foodflow','-At','-c',"SELECT count(*) FROM pg_tables WHERE schemaname='public'") -Label 'empty restore database').Trim()
      if($count -ne '0'){throw 'Restore database must be empty.'}
      $null=Invoke-RecoveryDocker -Arguments @('exec','-i',$db,'pg_restore','-U','foodflow','-d','foodflow','--exit-on-error','--single-transaction','--no-owner','--no-privileges') -Label 'transactional fresh database restore' -InputPath (Join-Path $directory 'database.dump')
      $null=Invoke-RecoveryDocker -Arguments ($restoreArgs+@('up','-d','--wait','--wait-timeout','90','api')) -Label 'private restored API'
      $api=(Invoke-RecoveryDocker -Arguments ($restoreArgs+@('ps','-q','api')) -Label 'restore API identity').Trim()
      $unpacked=Invoke-RecoveryDocker -Arguments @('exec','-i',$api,'/app/recovery','-mode','unpack','-files',[string]$manifest.snapshot.images.files,'-bytes',[string]$manifest.snapshot.images.bytes) -Label 'fresh photo volume restore' -InputPath (Join-Path $directory 'images.tar')
      $images=$unpacked|ConvertFrom-Json -AsHashtable
      foreach($key in @('files','bytes','sha256')){if($images[$key] -ne $manifest.snapshot.images[$key]){throw 'Restored photo content differs.'}}
      $comparison=Invoke-RecoveryDocker -Arguments @('exec','-i',$api,'/app/recovery','-mode','compare') -Label 'restored database and photo comparison' -InputText ($manifest.snapshot|ConvertTo-Json -Depth 20 -Compress)
      $verified=$comparison|ConvertFrom-Json
      if(!$verified.passed){throw 'Restore comparison did not pass.'}
      $timer.Stop()
      @{passed=$true;recorded_at=[DateTime]::UtcNow.ToString('o');project=$project;elapsed_seconds=[Math]::Round($timer.Elapsed.TotalSeconds,3);tables=$verified.tables;image_files=$verified.image_files;referenced_images=$verified.referenced_images;checks=@('artifact integrity before target creation','fresh isolated target','transactional PostgreSQL restore','empty photo volume restore','all table row fingerprints and sequence states equal','all photo paths and bytes equal','current photo references exist');limitations=@('same host with retained image IDs; no offsite/disaster claim','no host ports or worker; no automatic cutover','account probe is a separate fictional acceptance scenario')}|ConvertTo-Json -Depth 5|Set-Content -LiteralPath .cache/harness/learning-restore.json -Encoding utf8NoBOM
      Write-Output "PASS: restored data/photos verified in $project; targets retained and stopped."
    }
  }
}finally{
  try {
    if($restart.Count){$null=Invoke-RecoveryDocker -Arguments (@('start')+$restart) -Label 'resume original writers after failure';SourceReady}
    if($restoreArgs){$null=Invoke-RecoveryDocker -Arguments ($restoreArgs+@('stop')) -Label 'stop private restored target'}
  }finally{
    if($lock){$lock.Dispose();Remove-Item -LiteralPath $lockPath}
  }
}
