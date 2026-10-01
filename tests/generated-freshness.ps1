$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot '../scripts/compare-generated.ps1')
$repoTaskRoot=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$cacheTaskPrefix=[IO.Path]::GetFullPath((Join-Path $repoTaskRoot '.cache'))+[IO.Path]::DirectorySeparatorChar
$scratchTaskPath=Join-Path $cacheTaskPrefix ('generation-fixture-'+[guid]::NewGuid().ToString('N'))
$expected=Join-Path $scratchTaskPath 'expected'
$actual=Join-Path $scratchTaskPath 'actual'
New-Item -ItemType Directory -Path $expected,$actual -Force | Out-Null
function Must-Reject {
 $rejected=$false
 try{Assert-GeneratedFiles $expected $actual | Out-Null}catch{$rejected=$true}
 if(!$rejected){throw 'Stale or missing generated fixture was accepted'}
}
try {
 [IO.File]::WriteAllText((Join-Path $expected 'models.go'),"package fixture`n")
 [IO.File]::WriteAllText((Join-Path $actual 'models.go'),"package fixture`r`n")
 Assert-GeneratedFiles $expected $actual | Out-Null
 [IO.File]::WriteAllText((Join-Path $actual 'models.go'),"package stale`n")
 Must-Reject
 Rename-Item -LiteralPath (Join-Path $actual 'models.go') -NewName 'extra.go'
 Must-Reject
 [IO.File]::WriteAllText((Join-Path $actual 'models.go'),"package fixture`n")
 Must-Reject
 Write-Output 'PASS: stale content, missing/extra files rejected; CRLF equivalence accepted'
} finally {
 $resolvedTaskPath=[IO.Path]::GetFullPath($scratchTaskPath)
 if(!$resolvedTaskPath.StartsWith($cacheTaskPrefix,[StringComparison]::OrdinalIgnoreCase) -or [IO.Path]::GetFileName($resolvedTaskPath) -notmatch '^generation-fixture-[a-f0-9]{32}$'){throw 'Refused unsafe fixture cleanup'}
 Remove-Item -LiteralPath $resolvedTaskPath -Recurse -Force
}
