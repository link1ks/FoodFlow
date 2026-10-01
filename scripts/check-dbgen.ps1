$ErrorActionPreference='Stop'
$repoTaskRoot=[IO.Path]::GetFullPath((Split-Path $PSScriptRoot -Parent))
Set-Location -LiteralPath $repoTaskRoot
$cacheTaskRoot=Join-Path $repoTaskRoot '.cache'
$scratchTaskPath=Join-Path $cacheTaskRoot ('dbgen-check-'+[guid]::NewGuid().ToString('N'))
# Only repository schema/query/config inputs are copied; no database is opened.
New-Item -ItemType Directory -Path $scratchTaskPath -Force | Out-Null
try {
  Copy-Item -LiteralPath (Join-Path $repoTaskRoot 'sql') -Destination $scratchTaskPath -Recurse
  Copy-Item -LiteralPath (Join-Path $repoTaskRoot 'sqlc.yaml') -Destination $scratchTaskPath
  & go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate -f (Join-Path $scratchTaskPath 'sqlc.yaml')
  if($LASTEXITCODE -ne 0){throw 'sqlc generation failed'}
  $generatedTaskPath=Join-Path $scratchTaskPath 'internal/dbgen'
  $actualTaskPath=Join-Path $repoTaskRoot 'internal/dbgen'
  . (Join-Path $PSScriptRoot 'compare-generated.ps1')
  $count=Assert-GeneratedFiles $generatedTaskPath $actualTaskPath
  Write-Output "PASS: $count database generated files match pinned sqlc output"
} finally {
  # Never pass a computed recursive-delete target to another shell.
  $resolvedTaskPath=[IO.Path]::GetFullPath($scratchTaskPath)
  $checkedCachePrefix=[IO.Path]::GetFullPath($cacheTaskRoot)+[IO.Path]::DirectorySeparatorChar
  if(!$resolvedTaskPath.StartsWith($checkedCachePrefix,[StringComparison]::OrdinalIgnoreCase) -or [IO.Path]::GetFileName($resolvedTaskPath) -notmatch '^dbgen-check-[a-f0-9]{32}$'){throw 'Refused unsafe scratch cleanup path'}
  Remove-Item -LiteralPath $resolvedTaskPath -Recurse -Force
}
