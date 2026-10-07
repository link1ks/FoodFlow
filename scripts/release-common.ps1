# Release bundles contain only committed source and public runtime identities.
function Write-ReleaseSourceArchive([string]$Directory,[string]$Revision){
  $path=Join-Path $Directory 'source.zip'
  & git @('archive','--format=zip',('--output='+$path),$Revision)
  if($LASTEXITCODE -ne 0){throw 'Committed source archive failed.'}
  Set-RecoveryPrivatePath $path
}
function Get-ReleaseSchemaDigest([string]$Root){
  $facts=@(Get-ChildItem -LiteralPath (Join-Path $Root 'sql/schema') -File -Filter '*.sql' | Sort-Object Name | ForEach-Object {$_.Name+':'+(Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash})
  return [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes(($facts -join "`n")))).ToLowerInvariant()
}
function Assert-ReleaseVerified($Full,$Acceptance,[string]$Digest){
  foreach($pair in @(@($Full,'full'),@($Acceptance,'acceptance'))){
    $report=$pair[0]
    if($report.mode -ne $pair[1] -or $report.source_digest -ne $Digest -or !$report.integration_required -or !$report.checks.Count -or @($report.checks | Where-Object {!$_.passed}).Count){throw 'Release requires passing full and acceptance for this exact source.'}
    foreach($name in $(if($pair[1] -eq 'full'){@('architecture','repository-hygiene','knowledge','quality-map','release-regression','dbgen-regression','dbgen-check','format','vet','go-tests','web-api-check','web-test','web-build','source-stable')}else{@('architecture','repository-hygiene','knowledge','quality-map','read-recovery-regression','acceptance-images','platform','kitchen-flow','gateway-recovery','browser-fixture','browser','runtime-diagnostics','source-stable')})){
      if(@($report.checks | Where-Object {$_.name -eq $name -and $_.passed}).Count -ne 1){throw 'Release evidence is incomplete.'}
    }
  }
}
function Assert-ReleaseSourceArchive([string]$Path){
  $zip=[IO.Compression.ZipFile]::OpenRead($Path)
  try{
    $names=[Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase)
    foreach($entry in $zip.Entries){
      $name=$entry.FullName
      if($name.EndsWith('/')){continue}
      if($name.StartsWith('/') -or $name.Contains('\') -or $name.Contains(':') -or @($name.Split('/') | Where-Object {$_ -in @('','..','.')}).Count -or !$names.Add($name)){throw 'Unsafe source archive path.'}
      $leaf=($name -split '/')[-1]
      if(($leaf -like '.env*' -and $leaf -ne '.env.example') -or $name -match '(^|/)(\.cache|\.git|node_modules|data)/' -or $leaf -match '\.(dump|log|pem|key|pfx|p12)$'){throw 'Private or generated data must not enter source archives.'}
      # Git archives can encode symbolic links in their Unix mode.
      if((($entry.ExternalAttributes -shr 16) -band 0xf000) -eq 0xa000){throw 'Source archive must not contain symbolic links.'}
    }
    foreach($required in @('compose.learning.yaml','scripts/learning-deploy.ps1','README.md')){if(!$names.Contains($required)){throw 'Source archive is incomplete.'}}
  }finally{$zip.Dispose()}
}
function Assert-ReleaseBundle([string]$Directory){
  $folder=Get-Item -LiteralPath $Directory
  if(!$folder.PSIsContainer -or ($folder.Attributes -band [IO.FileAttributes]::ReparsePoint)){throw 'Release folder must be a real directory.'}
  $path=Join-Path $folder.FullName 'manifest.json';$null=Get-RecoveryArtifact $path
  $m=[IO.File]::ReadAllText($path)|ConvertFrom-Json -AsHashtable
  if($m.version -ne 1 -or $m.project -ne 'foodflow-learning' -or $m.revision -notmatch '^[a-f0-9]{40}$' -or $m.source_digest -notmatch '^[a-f0-9]{64}$' -or $m.schema_digest -notmatch '^[a-f0-9]{64}$' -or $m.history_digest -notmatch '^[a-f0-9]{64}$'){throw 'Invalid release identity.'}
  foreach($key in @('api','web','db')){if($m.images[$key] -notmatch '^sha256:[a-f0-9]{64}$'){throw 'Release image identity is invalid.'}}
  if($m.artifacts.Count -ne 2 -or !$m.artifacts.ContainsKey('source.zip') -or !$m.artifacts.ContainsKey('compose.learning.yaml')){throw 'Release artifact set is invalid.'}
  foreach($name in @('source.zip','compose.learning.yaml')){
    $expected=$m.artifacts[$name];$actual=Get-RecoveryArtifact (Join-Path $folder.FullName $name)
    if($expected.bytes -ne $actual.bytes -or $expected.sha256 -cne $actual.sha256){throw 'Release artifact integrity failed.'}
  }
  Assert-ReleaseSourceArchive (Join-Path $folder.FullName 'source.zip')
  return $m
}
function Assert-ReleaseCompatible($Manifest,[string]$Schema,[string]$History,[string]$Compose,[string]$DB){
  if($Manifest.schema_digest -cne $Schema -or $Manifest.history_digest -cne $History){throw 'Schema changed; direct application rollback refused. Restore into a fresh target instead.'}
  if($Manifest.artifacts['compose.learning.yaml'].sha256 -cne $Compose -or $Manifest.images.db -cne $DB){throw 'Deployment topology or database image changed; direct switch refused.'}
}
