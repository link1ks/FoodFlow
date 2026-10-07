$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot '../scripts/recovery-common.ps1')
. (Join-Path $PSScriptRoot '../scripts/release-common.ps1')
$root=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$scratch=Join-Path $root ('.cache/release-fixture-'+[guid]::NewGuid().ToString('N'))
New-RecoveryDirectory $scratch
function Reject([scriptblock]$Action){$failed=$false;try{& $Action|Out-Null}catch{$failed=$true};if(!$failed){throw 'Unsafe release fixture was accepted.'}}
function Archive([string[]]$Names){
  $path=Join-Path $scratch 'source.zip';if(Test-Path $path){Remove-Item -LiteralPath $path}
  $zip=[IO.Compression.ZipFile]::Open($path,[IO.Compression.ZipArchiveMode]::Create)
  try{foreach($name in $Names){$entry=$zip.CreateEntry($name);$stream=$entry.Open();try{$stream.WriteByte(65)}finally{$stream.Dispose()}}}finally{$zip.Dispose()}
}
try{
  Write-ReleaseSourceArchive $scratch 'HEAD'
  Assert-ReleaseSourceArchive (Join-Path $scratch 'source.zip')
  $base=@('README.md','compose.learning.yaml','scripts/learning-deploy.ps1','.env.example')
  Archive $base;Assert-ReleaseSourceArchive (Join-Path $scratch 'source.zip')
  foreach($bad in @('.env','.env.local','data/photos/a.webp','.cache/learning.env','secret.pem','a/../b','a\b','a:/b','README.MD')){
    Archive ($base+@($bad));Reject {Assert-ReleaseSourceArchive (Join-Path $scratch 'source.zip')}
  }
  Archive $base
  [IO.File]::WriteAllText((Join-Path $scratch 'compose.learning.yaml'),'fixture')
  $hash='a'*64;$image='sha256:'+('b'*64)
  $m=@{version=1;project='foodflow-learning';revision=('c'*40);source_digest=$hash;schema_digest=$hash;history_digest=$hash;images=@{api=$image;web=$image;db=$image};artifacts=@{}}
  foreach($name in @('source.zip','compose.learning.yaml')){$m.artifacts[$name]=Get-RecoveryArtifact (Join-Path $scratch $name)}
  Write-RecoveryPrivateJSON (Join-Path $scratch 'manifest.json') $m
  $null=Assert-ReleaseBundle $scratch
  Assert-ReleaseCompatible $m $hash $hash $m.artifacts['compose.learning.yaml'].sha256 $image
  Reject {Assert-ReleaseCompatible $m ('d'*64) $hash $m.artifacts['compose.learning.yaml'].sha256 $image}
  Reject {Assert-ReleaseCompatible $m $hash ('d'*64) $m.artifacts['compose.learning.yaml'].sha256 $image}
  Reject {Assert-ReleaseCompatible $m $hash $hash ('d'*64) $image}
  Reject {Assert-ReleaseCompatible $m $hash $hash $m.artifacts['compose.learning.yaml'].sha256 ('sha256:'+('d'*64))}
  [IO.File]::AppendAllText((Join-Path $scratch 'compose.learning.yaml'),'corrupt')
  Reject {Assert-ReleaseBundle $scratch}
  $full=@{mode='full';source_digest=$hash;integration_required=$true;checks=@(@('architecture','knowledge','quality-map','release-regression','dbgen-regression','dbgen-check','format','vet','go-tests','web-api-check','web-test','web-build','source-stable')|ForEach-Object {@{name=$_;passed=$true}})}
  $accept=@{mode='acceptance';source_digest=$hash;integration_required=$true;checks=@(@('architecture','knowledge','quality-map','read-recovery-regression','acceptance-images','platform','kitchen-flow','gateway-recovery','browser-fixture','browser','runtime-diagnostics','source-stable')|ForEach-Object {@{name=$_;passed=$true}})}
  Assert-ReleaseVerified $full $accept $hash
  Reject {Assert-ReleaseVerified $full $accept ('d'*64)}
  $full.checks[0].passed=$false;Reject {Assert-ReleaseVerified $full $accept $hash};$full.checks[0].passed=$true
  $full.checks=@($full.checks|Where-Object {$_.name -ne 'web-build'});Reject {Assert-ReleaseVerified $full $accept $hash}
  Write-Output 'PASS: private/traversal/duplicate archives, corruption, schema/history/topology/image drift and incomplete verification rejected.'
}finally{
  $resolved=[IO.Path]::GetFullPath($scratch)
  if($resolved.StartsWith((Join-Path $root '.cache')+[IO.Path]::DirectorySeparatorChar) -and (Split-Path $resolved -Leaf) -like 'release-fixture-*'){
    Remove-Item -LiteralPath $resolved -Recurse -Force
  }
}
