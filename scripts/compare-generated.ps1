function Assert-GeneratedFiles([string]$ExpectedPath,[string]$ActualPath) {
  $names=@(Get-ChildItem -LiteralPath $ExpectedPath -File -Recurse | ForEach-Object {[IO.Path]::GetRelativePath($ExpectedPath,$_.FullName)})
  $actualNames=@(Get-ChildItem -LiteralPath $ActualPath -File -Recurse | ForEach-Object {[IO.Path]::GetRelativePath($ActualPath,$_.FullName)})
  if(Compare-Object $names $actualNames -CaseSensitive){throw 'Generated database file inventory differs; run sqlc generate'}
  foreach($name in $names){
    $generated=[IO.File]::ReadAllText((Join-Path $ExpectedPath $name)).Replace("`r`n","`n")
    $actual=[IO.File]::ReadAllText((Join-Path $ActualPath $name)).Replace("`r`n","`n")
    if($generated -cne $actual){throw "Generated database file is stale: $name; run sqlc generate"}
  }
  return $names.Count
}
