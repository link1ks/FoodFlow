# Fictional recovery acceptance. Credentials and tokens stay in memory/stdin.
$ErrorActionPreference='Stop'
Set-Location -LiteralPath (Split-Path $PSScriptRoot -Parent)
. (Join-Path (Split-Path $PSScriptRoot -Parent) 'scripts/recovery-common.ps1')
$script:RecoveryProcessEnv=Read-RecoveryCredentials
function Check($ok,$message){if(!$ok){throw $message}}
$base='http://127.0.0.1:17173/api'
function Call($method,$path,$body=$null,$token='',$key=''){
  $headers=@{};if($token){$headers.Authorization='Bearer '+$token};if($key){$headers['Idempotency-Key']=$key}
  $args=@{Method=$method;Uri=$base+$path;Headers=$headers;ContentType='application/json';TimeoutSec=15}
  if($null -ne $body){$args.Body=$body|ConvertTo-Json -Depth 10 -Compress}
  Invoke-RestMethod @args
}
# Regression: absent process variables must remain absent after deploy on Linux.
$saved=@{};foreach($key in @('POSTGRES_PASSWORD','JWT_SECRET')){$saved[$key]=[Environment]::GetEnvironmentVariable($key,'Process');Remove-Item -LiteralPath ('Env:'+$key) -ErrorAction SilentlyContinue}
try {
  & ./scripts/learning-deploy.ps1 status | Out-Null
  foreach($key in $saved.Keys){Check (!(Test-Path -LiteralPath ('Env:'+$key))) 'Deploy left an empty process credential override.'}
}finally{
  foreach($key in $saved.Keys){if($null -eq $saved[$key]){Remove-Item -LiteralPath ('Env:'+$key) -ErrorAction SilentlyContinue}else{[Environment]::SetEnvironmentVariable($key,$saved[$key],'Process')}}
}
$email='recovery-'+[guid]::NewGuid().ToString('N')+'@example.test'
$password='Recovery-Fixture-2026!'
$account=Call POST /register @{email=$email;password=$password;name='恢复验收'}
$household=Call POST /households @{name='恢复验收';servings=2} $account.token
$root='/households/'+$household.id
$ingredient=Call POST ($root+'/ingredients') @{name='番茄';category='蔬菜';unit='g'} $account.token
$stock=@{ingredient_id=$ingredient.id;quantity='300';reason='manual';source='recovery-fixture'}
$stockKey=[guid]::NewGuid().ToString()
$null=Call POST ($root+'/stock') $stock $account.token $stockKey
$catalog=Get-Content web/src/catalogPhotos.json -Raw|ConvertFrom-Json
$photoEntry=$catalog|Where-Object name -eq '番茄'|Select-Object -First 1
$photo=Get-Item -LiteralPath ('web/public'+$photoEntry.src)
$photoSHA=(Get-FileHash -LiteralPath $photo.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
$null=Invoke-RestMethod -Method Post -Uri ($base+$root+'/ingredients/'+$ingredient.id+'/image') -Headers @{Authorization='Bearer '+$account.token} -Form @{image=$photo} -TimeoutSec 15
$proof=@{Email=$email;Password=$password;Household=$household.id;Ingredient=$ingredient.id;PhotoSHA256=$photoSHA;OldToken=$account.token;StockKey=$stockKey;Stock=$stock}
$restoreArgs=$null
try {
  & ./scripts/learning-recovery.ps1 -Action backup
  $backup=Get-Content .cache/harness/learning-backup.json -Raw|ConvertFrom-Json
  Check $backup.passed 'Backup did not pass.'
  & ./scripts/learning-recovery.ps1 -Action restore-check -Backup $backup.backup
  $restored=Get-Content .cache/harness/learning-restore.json -Raw|ConvertFrom-Json
  Check $restored.passed 'Restore did not pass.'
  $config=Join-Path $PWD ('.cache/learning-restores/'+$restored.project+'.env')
  foreach($line in [IO.File]::ReadAllLines($config)){$parts=$line -split '=',2;Check ($parts.Count -eq 2) 'Invalid private restore configuration.';$script:RecoveryProcessEnv[$parts[0]]=$parts[1]}
  $restoreArgs=@('compose','--env-file',$config,'-f','compose.restore.yaml')
  $null=Invoke-RecoveryDocker -Arguments ($restoreArgs+@('up','-d','--wait','--wait-timeout','90','api')) -Label 'restart fictional recovery target'
  $api=(Invoke-RecoveryDocker -Arguments ($restoreArgs+@('ps','-q','api')) -Label 'fictional restore identity').Trim()
  $probe=Invoke-RecoveryDocker -Arguments @('exec','-i',$api,'/app/recovery','-mode','probe') -InputText ($proof|ConvertTo-Json -Depth 10 -Compress) -Label 'fictional recovered account acceptance'
  $result=$probe|ConvertFrom-Json
  Check $result.passed 'Recovered account probe did not pass.'
  $inventory=Call GET ($root+'/inventory') $null $account.token
  Check ($inventory.items.Count -eq 1 -and $inventory.items[0].quantity -eq '300.000') 'Original source stock changed during restore.'
  $response=Invoke-WebRequest -Uri ($base+$root+'/ingredients/'+$ingredient.id+'/image') -Headers @{Authorization='Bearer '+$account.token} -TimeoutSec 15
  Check ([Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([byte[]]$response.Content)).ToLowerInvariant() -eq $photoSHA) 'Original source photo changed during restore.'
  # Corrupt a new private copy, never the genuine backup or a database volume.
  $bad=Join-Path $PWD ('.cache/learning-backups/corrupt-fixture-'+[guid]::NewGuid().ToString('N'))
  New-RecoveryDirectory $bad;Set-RecoveryPrivatePath $bad $true # Repeated ACL application regression.
  foreach($file in @('manifest.json','database.dump','images.tar')){Copy-Item -LiteralPath (Join-Path $backup.backup $file) -Destination (Join-Path $bad $file);Set-RecoveryPrivatePath (Join-Path $bad $file)}
  $path=Join-Path $bad 'images.tar';$stream=[IO.File]::Open($path,[IO.FileMode]::Open,[IO.FileAccess]::ReadWrite,[IO.FileShare]::None)
  try{$byte=$stream.ReadByte();$stream.Position=0;$stream.WriteByte([byte]($byte -bxor 1))}finally{$stream.Dispose()}
  $before=@(Get-ChildItem -LiteralPath .cache/learning-restores -Filter '*.env').Count
  $rejected=$false
  try{& ./scripts/learning-recovery.ps1 -Action restore-check -Backup $bad | Out-Null}catch{$rejected=$true}
  Check $rejected 'Corrupt backup was accepted.'
  Check (@(Get-ChildItem -LiteralPath .cache/learning-restores -Filter '*.env').Count -eq $before) 'Corrupt backup created a recovery target.'
  Check ((Invoke-WebRequest http://127.0.0.1:17173/health/ready -TimeoutSec 10).StatusCode -eq 200) 'Original service unavailable.'
  @{passed=$true;recorded_at=[DateTime]::UtcNow.ToString('o');tables=$restored.tables;image_files=$restored.image_files;writer_pause_seconds=$backup.writer_pause_seconds;restore_seconds=$restored.elapsed_seconds;checks=@($result.checks)+@('original token/stock/photo preserved','corrupt artifact rejected before target creation','repeated private directory permissions','absent credential environment restored');limitations=@('fictional accounts; no real model or SMS','same host with cached images; not offsite disaster recovery','source fixture and stopped restore targets retained')}|ConvertTo-Json -Depth 5|Set-Content -LiteralPath .cache/harness/learning-recovery.json -Encoding utf8NoBOM
  Write-Output 'PASS: fictional account/inventory/photo/idempotency recovery and corruption guards; redacted evidence saved.'
}finally{
  if($restoreArgs){$null=Invoke-RecoveryDocker -Arguments ($restoreArgs+@('stop')) -Label 'stop fictional recovery target'}
}
