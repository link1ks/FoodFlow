$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot '../scripts/update-common.ps1')
$root=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$scratch=Join-Path $root ('.cache/update-fixture-'+[guid]::NewGuid().ToString('N'))
$null=New-Item -ItemType Directory -Path $scratch
$remote=Join-Path $scratch 'remote.git'
$seed=Join-Path $scratch 'seed'
$client=Join-Path $scratch 'client with spaces'
function G([string]$Path,[string[]]$Arguments) {
  $ErrorActionPreference='Continue'
  $output=@(& git -C $Path @Arguments 2>&1)
  if ($LASTEXITCODE -ne 0) { throw "Local fixture Git $($Arguments[0]) failed: $($output -join ' ')" }
  return ($output -join "`n").Trim()
}
function Assert([bool]$OK,[string]$Message) { if (!$OK) { throw $Message } }
function Reject([scriptblock]$Action,[string]$Pattern) {
  $message=''
  try { & $Action | Out-Null } catch { $message=$_.Exception.Message }
  Assert ($message -like $Pattern) "Expected rejection '$Pattern', got '$message'."
}
function Commit([string]$Path,[string]$Message) {
  $null=G $Path @('-c','user.name=Updater Fixture','-c','user.email=fixture@example.invalid','commit','-am',$Message)
}
try {
  $null=G $scratch @('init','--bare',$remote)
  $null=G $scratch @('init','-b','main',$seed)
  $null=G $seed @('config','core.autocrlf','false')
  [IO.File]::WriteAllText((Join-Path $seed '.gitignore'), ".env`ndata/`n")
  [IO.File]::WriteAllText((Join-Path $seed 'version.txt'), 'v1')
  $null=G $seed @('add','.')
  Commit $seed 'initial'
  $null=G $seed @('remote','add','origin',$remote)
  $null=G $seed @('push','-u','origin','main')
  $null=G $remote @('symbolic-ref','HEAD','refs/heads/main')
  $null=G $scratch @('clone',$remote,$client)
  [IO.File]::WriteAllText((Join-Path $client '.env'), 'FICTIONAL_SECRET=retain')
  $null=New-Item -ItemType Directory -Path (Join-Path $client 'data')
  [IO.File]::WriteAllText((Join-Path $client 'data/photo.txt'), 'retain photo')
  $initial=G $client @('rev-parse','HEAD')
  [IO.File]::WriteAllText((Join-Path $seed 'version.txt'), 'v2')
  Commit $seed 'remote update'
  $null=G $seed @('push')
  Invoke-FoodFlowUpdate $client check
  Assert ((G $client @('rev-parse','HEAD')) -eq $initial) 'Check modified the worktree.'
  Invoke-FoodFlowUpdate $client update
  $updated=G $client @('rev-parse','HEAD')
  Assert ($updated -eq (G $seed @('rev-parse','HEAD'))) 'Fast-forward did not reach the remote commit.'
  Assert ([IO.File]::ReadAllText((Join-Path $client '.env')) -eq 'FICTIONAL_SECRET=retain') 'Configuration was changed.'
  Assert ([IO.File]::ReadAllText((Join-Path $client 'data/photo.txt')) -eq 'retain photo') 'Photo was changed.'
  Invoke-FoodFlowUpdate $client update
  Assert ((G $client @('rev-parse','HEAD')) -eq $updated) 'Up-to-date update changed HEAD.'

  # Test the Compose boundary without touching an installed Docker daemon or user's data.
  $script:dockerCalls=@()
  $script:dockerMode='pass'
  function docker {
    $script:dockerCalls+=,@($args)
    $global:LASTEXITCODE=0
    if (($script:dockerMode -eq 'offline' -and $args[0] -eq 'info') -or
        ($script:dockerMode -eq 'build-failure' -and $args -contains 'up')) { $global:LASTEXITCODE=1 }
  }
  try {
    Invoke-FoodFlowUpdate $client update -Restart
    Assert ($script:dockerCalls.Count -eq 3) 'Already-current source did not restart Compose.'
    $composeArgs=$script:dockerCalls[2]
    Assert ($composeArgs -contains $client) 'Compose project directory was not pinned.'
    Assert ($composeArgs -contains (Join-Path $client 'compose.yaml')) 'Default Compose file was not pinned.'
    Assert ($composeArgs -contains '--wait' -and $composeArgs -contains '--build') 'Compose did not build and wait.'
    Assert (!($composeArgs -contains 'down') -and !($composeArgs -contains '-v')) 'Compose used destructive volume flags.'
    $script:dockerMode='offline'
    Reject { Invoke-FoodFlowUpdate $client update -Restart } '*Start Docker Desktop*'
    Assert ((G $client @('rev-parse','HEAD')) -eq $updated) 'Docker preflight failure changed source.'
    $script:dockerMode='build-failure'
    Reject { Invoke-FoodFlowUpdate $client update -Restart } '*Docker startup failed*'
    $script:dockerMode='pass'
    Invoke-FoodFlowUpdate $client update -Restart
  } finally { Remove-Item -LiteralPath Function:docker }

  [IO.File]::WriteAllText((Join-Path $client 'version.txt'), 'local edits')
  Invoke-FoodFlowUpdate $client check
  Reject { Invoke-FoodFlowUpdate $client update } '*Local changes*'
  Assert ([IO.File]::ReadAllText((Join-Path $client 'version.txt')) -eq 'local edits') 'Dirty source was overwritten.'
  [IO.File]::WriteAllText((Join-Path $client 'version.txt'), 'v2')
  [IO.File]::WriteAllText((Join-Path $client 'notes.txt'), 'untracked')
  Reject { Invoke-FoodFlowUpdate $client update } '*untracked files*'
  Remove-Item -LiteralPath (Join-Path $client 'notes.txt')

  $gitDir=G $client @('rev-parse','--absolute-git-dir')
  $lock=[IO.File]::Open((Join-Path $gitDir 'foodflow-update.lock'),[IO.FileMode]::OpenOrCreate,[IO.FileAccess]::ReadWrite,[IO.FileShare]::None)
  try { Reject { Invoke-FoodFlowUpdate $client update } '*Another FoodFlow updater*' } finally { $lock.Dispose() }
  [IO.File]::WriteAllText((Join-Path $gitDir 'MERGE_HEAD'), $updated)
  Reject { Invoke-FoodFlowUpdate $client update } '*in-progress Git operation*'
  Remove-Item -LiteralPath (Join-Path $gitDir 'MERGE_HEAD')

  $null=G $client @('checkout','--detach')
  Reject { Invoke-FoodFlowUpdate $client update } '*Detached HEAD*'
  $null=G $client @('checkout','main')
  $null=G $client @('config','--unset','branch.main.remote')
  Reject { Invoke-FoodFlowUpdate $client update } '*No remote upstream*'
  $null=G $client @('config','branch.main.remote','origin')

  [IO.File]::WriteAllText((Join-Path $client 'version.txt'), 'local commit')
  Commit $client 'local-only commit'
  $localHead=G $client @('rev-parse','HEAD')
  Reject { Invoke-FoodFlowUpdate $client update } '*Local commits differ*'
  [IO.File]::WriteAllText((Join-Path $seed 'version.txt'), 'v3')
  Commit $seed 'divergent remote'
  $null=G $seed @('push')
  Reject { Invoke-FoodFlowUpdate $client update } '*Local commits differ*'
  Assert ((G $client @('rev-parse','HEAD')) -eq $localHead) 'Local commit was discarded.'

  $clean=Join-Path $scratch 'clean client'
  $null=G $scratch @('clone',$remote,$clean)
  $cleanHead=G $clean @('rev-parse','HEAD')
  $null=G $clean @('remote','set-url','origin',(Join-Path $scratch 'missing remote.git'))
  Reject { Invoke-FoodFlowUpdate $clean update } '*Git fetch failed*'
  Assert ((G $clean @('rev-parse','HEAD')) -eq $cleanHead) 'Failed fetch changed source.'
  $null=G $clean @('remote','set-url','origin',$remote)
  Reject { Invoke-FoodFlowUpdate $clean update -Restart } '*Configure .env*'
  Assert ((G $clean @('rev-parse','HEAD')) -eq $cleanHead) 'Restart preflight changed source.'

  [IO.File]::WriteAllText((Join-Path $seed '.env'), 'remote config must not replace private config')
  $null=G $seed @('add','-f','.env')
  Commit $seed 'unsafe remote config'
  $null=G $seed @('push')
  [IO.File]::WriteAllText((Join-Path $clean '.env'), 'retain private config')
  Reject { Invoke-FoodFlowUpdate $clean update } '*local configuration/data path*'
  Assert ([IO.File]::ReadAllText((Join-Path $clean '.env')) -eq 'retain private config') 'Ignored config was overwritten.'
  Assert ((G $clean @('rev-parse','HEAD')) -eq $cleanHead) 'Rejected remote changed HEAD.'
  Write-Output 'PASS: check, fast-forward, repeat update, configuration/photos, dirty/untracked files, lock, active Git operation, detached/missing upstream, ahead/diverged history, fetch failure, restart preflight, Compose arguments/retry (mocked) and unsafe remote paths.'
} finally {
  $resolved=[IO.Path]::GetFullPath($scratch)
  $allowed=[IO.Path]::GetFullPath((Join-Path $root '.cache'))+[IO.Path]::DirectorySeparatorChar
  if ($resolved.StartsWith($allowed) -and (Split-Path $resolved -Leaf) -like 'update-fixture-*') {
    Remove-Item -LiteralPath $resolved -Recurse -Force
  }
}
