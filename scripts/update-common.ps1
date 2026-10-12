# Shared by the updater and offline regression fixtures. Windows PowerShell 5.1 compatible.
function Invoke-UpdateGit {
  param([string]$Root, [string[]]$Arguments)
  # Windows PowerShell 5.1 represents redirected native stderr as ErrorRecords.
  # Capture them instead of letting them bypass the sanitized failure message.
  $ErrorActionPreference = 'Continue'
  $output = @(& git -C $Root @Arguments 2>&1)
  if ($LASTEXITCODE -ne 0) {
    # Native output can contain credential-bearing remote URLs. Keep errors bounded and private.
    throw "Git $($Arguments[0]) failed. Check Git access/network and the repository configuration."
  }
  return ($output -join "`n").Trim()
}

function Assert-UpdateDocker {
  $ErrorActionPreference = 'Continue'
  & docker info --format '{{.ServerVersion}}' 2>&1 | Out-Null
  if ($LASTEXITCODE -ne 0) { throw 'Start Docker Desktop before using -Restart; source has not changed.' }
  & docker compose version 2>&1 | Out-Null
  if ($LASTEXITCODE -ne 0) { throw 'Docker Compose is required; source has not changed.' }
}

function Assert-UpdateClean([string]$Root) {
  $dirty = Invoke-UpdateGit $Root @('status', '--porcelain', '--untracked-files=all')
  if ($dirty) { throw 'Local changes or untracked files exist. Commit or move them before updating; nothing was overwritten.' }
}

function Get-UpdateState([string]$Root) {
  try { $branch = Invoke-UpdateGit $Root @('symbolic-ref', '--quiet', '--short', 'HEAD') }
  catch { throw 'Detached HEAD cannot be updated. Switch to a local branch, such as main.' }
  try {
    $remote = Invoke-UpdateGit $Root @('config', '--get', "branch.$branch.remote")
    $mergeRef = Invoke-UpdateGit $Root @('config', '--get', "branch.$branch.merge")
  } catch { throw 'No remote upstream is configured. Configure the branch upstream before updating.' }
  if (!$remote -or $remote -eq '.' -or $remote.StartsWith('-') -or !$mergeRef.StartsWith('refs/heads/')) {
    throw 'Use a local branch tracking a remote branch, such as main tracking origin/main.'
  }
  $remoteBranch = $mergeRef.Substring('refs/heads/'.Length)
  $trackingRef = "refs/remotes/$remote/$remoteBranch"
  # Fetch only the tracked branch. A force push is reflected in the tracking ref, then rejected
  # below if applying it would discard local commits. No worktree changes happen during fetch.
  $null = Invoke-UpdateGit $Root @('fetch', '--no-tags', $remote, "+${mergeRef}:${trackingRef}")
  $current = Invoke-UpdateGit $Root @('rev-parse', 'HEAD')
  $target = Invoke-UpdateGit $Root @('rev-parse', '--verify', "${trackingRef}^{commit}")
  $counts = (Invoke-UpdateGit $Root @('rev-list', '--left-right', '--count', "$current...$target")) -split '\s+'
  return [pscustomobject]@{ Branch=$branch; Current=$current; Target=$target; Ahead=[int]$counts[0]; Behind=[int]$counts[1] }
}

function Invoke-FoodFlowUpdate {
  param([string]$Root, [ValidateSet('check','update')][string]$Action='update', [switch]$Restart)
  if ($Restart -and $Action -ne 'update') { throw '-Restart is only supported with update.' }
  $Root = [IO.Path]::GetFullPath($Root)
  if (!(Get-Command git -ErrorAction SilentlyContinue)) { throw 'Install Git first: https://git-scm.com/downloads' }
  try { $top = Invoke-UpdateGit $Root @('rev-parse', '--show-toplevel') }
  catch { throw 'This folder is not a Git clone. Use git clone instead of Download ZIP; see docs/UPDATING.md.' }
  if ([IO.Path]::GetFullPath($top) -ne $Root) { throw 'Run the updater from the FoodFlow repository root.' }
  $gitDir = Invoke-UpdateGit $Root @('rev-parse', '--absolute-git-dir')
  foreach ($operation in @('MERGE_HEAD','CHERRY_PICK_HEAD','REVERT_HEAD','rebase-merge','rebase-apply','BISECT_START')) {
    if (Test-Path -LiteralPath (Join-Path $gitDir $operation)) { throw 'Finish the in-progress Git operation before checking or applying updates.' }
  }
  $lock = $null
  try {
    try { $lock = [IO.File]::Open((Join-Path $gitDir 'foodflow-update.lock'), [IO.FileMode]::OpenOrCreate, [IO.FileAccess]::ReadWrite, [IO.FileShare]::None) }
    catch {
      $code = $_.Exception.GetBaseException().HResult -band 0xffff
      if ($code -in @(11,32,33)) { throw 'Another FoodFlow updater is running. Wait for it to finish.' }
      throw 'Cannot acquire the update lock. Check write permissions for the repository Git directory.'
    }
    if ($Action -eq 'update') { Assert-UpdateClean $Root }
    if ($Restart) {
      if (!(Test-Path -LiteralPath (Join-Path $Root '.env') -PathType Leaf)) { throw 'Configure .env before using -Restart.' }
      if (!(Get-Command docker -ErrorAction SilentlyContinue)) { throw 'Install and start Docker Desktop before using -Restart.' }
      Assert-UpdateDocker
    }
    $state = Get-UpdateState $Root
    Write-Output "Branch: $($state.Branch); local: $($state.Current.Substring(0,8)); remote: $($state.Target.Substring(0,8))"
    Write-Output "Remote updates: $($state.Behind); local-only commits: $($state.Ahead)"
    if ($Action -eq 'check') { return }
    if ($state.Ahead -gt 0) { throw 'Local commits differ from the remote. Resolve them manually; automatic update will not reset or rebase them.' }
    if ($state.Behind -gt 0) {
      # A remote must never introduce private configuration/data paths into the worktree.
      $paths = (Invoke-UpdateGit $Root @('ls-tree', '-r', '--name-only', $state.Target)) -split "`n"
      foreach ($path in $paths) {
        if ($path -match '^(\.env($|\.)|\.cache(/|$)|data(/|$))' -and $path -ne '.env.example') {
          throw 'Remote contains a local configuration/data path. Update rejected to preserve local data.'
        }
      }
      Assert-UpdateClean $Root
      if ((Invoke-UpdateGit $Root @('rev-parse', 'HEAD')) -ne $state.Current -or
          (Invoke-UpdateGit $Root @('symbolic-ref', '--quiet', '--short', 'HEAD')) -ne $state.Branch) {
        throw 'Branch changed while checking updates. Retry after other Git operations finish.'
      }
      $null = Invoke-UpdateGit $Root @('merge', '--ff-only', $state.Target)
      Write-Output 'Source updated successfully. Local configuration and Docker data volumes are preserved.'
    } else { Write-Output 'Already up to date.' }
    if ($Restart) {
      # Pin project identity to this repository, even if the launcher was invoked from elsewhere.
      Push-Location -LiteralPath $Root
      try {
        & docker compose --project-directory $Root -f (Join-Path $Root 'compose.yaml') up --build -d --wait --wait-timeout 180
        if ($LASTEXITCODE -ne 0) { throw 'Source is updated, but Docker startup failed. Fix Docker and retry with -Restart. Do not delete volumes.' }
      } finally { Pop-Location }
      Write-Output 'FoodFlow is ready: http://localhost:5173'
    } elseif ($state.Behind -gt 0) {
      Write-Output 'Running services still use the previous build. Restart your development process or rebuild your existing deployment.'
    }
  } finally { if ($lock) { $lock.Dispose() } }
}
