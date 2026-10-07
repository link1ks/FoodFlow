# Shared private-file and binary-safe Docker I/O for the learning recovery tools.
function Set-RecoveryPrivatePath([string]$Path,[bool]$Directory=$false){
  $item=Get-Item -LiteralPath $Path
  if($item.Attributes -band [IO.FileAttributes]::ReparsePoint){throw 'Recovery paths must not be links.'}
  if($IsWindows){
    $identity=[Security.Principal.WindowsIdentity]::GetCurrent().User
    if($Directory){
      $rule=[Security.AccessControl.FileSystemAccessRule]::new($identity,'FullControl','ContainerInherit,ObjectInherit','None','Allow')
    }else{
      $rule=[Security.AccessControl.FileSystemAccessRule]::new($identity,'FullControl','Allow')
    }
    $existing=Get-Acl -LiteralPath $Path
    $rules=@($existing.GetAccessRules($true,$true,[Security.Principal.SecurityIdentifier]))
    $alreadyPrivate=$existing.AreAccessRulesProtected -and $rules.Count -eq 1 -and $rules[0].IdentityReference -eq $identity -and $rules[0].AccessControlType -eq [Security.AccessControl.AccessControlType]::Allow -and $rules[0].FileSystemRights -eq [Security.AccessControl.FileSystemRights]::FullControl
    if(!$alreadyPrivate){
      $existing.SetAccessRuleProtection($true,$false)
      foreach($oldRule in @($existing.Access)){$null=$existing.RemoveAccessRuleAll($oldRule)}
      $existing.SetAccessRule($rule)
      Set-Acl -LiteralPath $Path -AclObject $existing
    }
  }else{
    $mode=[IO.UnixFileMode]::UserRead -bor [IO.UnixFileMode]::UserWrite
    if($Directory){$mode=$mode -bor [IO.UnixFileMode]::UserExecute}
    [IO.File]::SetUnixFileMode($Path,$mode)
  }
}
function New-RecoveryDirectory([string]$Path){
  New-Item -ItemType Directory -Path $Path -Force | Out-Null
  Set-RecoveryPrivatePath $Path $true
}
function Write-RecoveryPrivateJSON([string]$Path,$Value){
  $stream=[IO.File]::Open($Path,[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None)
  $stream.Dispose();Set-RecoveryPrivatePath $Path
  [IO.File]::WriteAllText($Path,($Value|ConvertTo-Json -Depth 20)+"`n")
}
function Read-RecoveryCredentials {
  $path=Join-Path $PWD '.cache/learning.env'
  $values=@{}
  foreach($line in [IO.File]::ReadAllLines($path)){
    if($line -match '^(POSTGRES_PASSWORD|JWT_SECRET)=([A-Fa-f0-9]{48,})$'){
      if($values.ContainsKey($Matches[1])){throw 'Duplicate learning credential key.'}
      $values[$Matches[1]]=$Matches[2]
    }elseif($line.Trim() -ne ''){throw 'Invalid generated learning configuration.'}
  }
  if($values.Count -ne 2 -or $values.JWT_SECRET.Length -lt 64){throw 'Incomplete learning credentials.'}
  return $values
}
function Invoke-RecoveryDocker {
  param([string[]]$Arguments,[string]$Label='operation',[string]$InputPath='',
        [AllowNull()][string]$InputText=$null,[string]$OutputPath='')
  # BaseStream avoids PowerShell text conversion of PostgreSQL/TAR archives.
  # ArgumentList avoids shell interpolation. Stderr is drained but never exported.
  $psi=[Diagnostics.ProcessStartInfo]::new('docker')
  $psi.UseShellExecute=$false;$psi.CreateNoWindow=$true
  $psi.RedirectStandardOutput=$true;$psi.RedirectStandardError=$true
  $hasInput=($InputPath -ne '' -or $null -ne $InputText)
  $psi.RedirectStandardInput=$hasInput
  foreach($argument in $Arguments){$psi.ArgumentList.Add($argument)}
  foreach($key in $script:RecoveryProcessEnv.Keys){$psi.Environment[$key]=$script:RecoveryProcessEnv[$key]}
  $process=[Diagnostics.Process]::new();$process.StartInfo=$psi
  $inputStream=$null;$outputStream=$null;$started=$false
  try {
    if($InputPath){$inputStream=[IO.File]::OpenRead($InputPath)}
    elseif($null -ne $InputText){$inputStream=[IO.MemoryStream]::new([Text.Encoding]::UTF8.GetBytes($InputText))}
    if($OutputPath){
      $outputStream=[IO.File]::Open($OutputPath,[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None)
      Set-RecoveryPrivatePath $OutputPath
    }
    if(!$process.Start()){throw "Recovery Docker step could not start: $Label"}
    $started=$true
    $drain=$process.StandardError.BaseStream.CopyToAsync([IO.Stream]::Null)
    if($outputStream){$outputTask=$process.StandardOutput.BaseStream.CopyToAsync($outputStream)}
    else{$outputTask=$process.StandardOutput.ReadToEndAsync()}
    if($inputStream){
      $inputTask=$inputStream.CopyToAsync($process.StandardInput.BaseStream)
      if(!$inputTask.Wait(600000)){throw "Recovery input deadline exceeded: $Label"}
      $process.StandardInput.Close()
    }
    if(!$process.WaitForExit(600000)){throw "Recovery Docker deadline exceeded: $Label"}
    $null=$drain.GetAwaiter().GetResult()
    if($process.ExitCode -ne 0){throw "Recovery Docker step failed: $Label"}
    if($outputStream){$null=$outputTask.GetAwaiter().GetResult();return}
    return $outputTask.GetAwaiter().GetResult()
  }finally{
    if($started -and !$process.HasExited){$process.Kill($true)}
    if($inputStream){$inputStream.Dispose()};if($outputStream){$outputStream.Dispose()}
    $process.Dispose()
  }
}
function Get-RecoveryArtifact([string]$Path){
  $file=Get-Item -LiteralPath $Path
  if($file.PSIsContainer -or ($file.Attributes -band [IO.FileAttributes]::ReparsePoint)){throw 'Backup artifacts must be ordinary files.'}
  return @{bytes=$file.Length;sha256=(Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()}
}
function Assert-RecoveryBundle([string]$Directory){
  $folder=Get-Item -LiteralPath $Directory
  if(!$folder.PSIsContainer -or ($folder.Attributes -band [IO.FileAttributes]::ReparsePoint)){throw 'Backup directory must be real.'}
  $manifestPath=Join-Path $folder.FullName 'manifest.json'
  $null=Get-RecoveryArtifact $manifestPath
  $manifest=[IO.File]::ReadAllText($manifestPath)|ConvertFrom-Json -AsHashtable
  if($manifest.version -ne 1 -or $manifest.source_project -ne 'foodflow-learning' -or
     $manifest.snapshot.version -ne 1 -or $manifest.snapshot.postgres_major -ne 16 -or
     $manifest.api_image -notmatch '^sha256:[a-f0-9]{64}$' -or $manifest.db_image -notmatch '^sha256:[a-f0-9]{64}$' -or
     $manifest.snapshot.images.files -lt 0 -or $manifest.snapshot.images.bytes -lt 0 -or
     $manifest.snapshot.images.sha256 -notmatch '^[a-f0-9]{64}$'){
    throw 'Unsupported or invalid backup manifest.'
  }
  foreach($name in @('database.dump','images.tar')){
    $expected=$manifest.artifacts[$name]
    $actual=Get-RecoveryArtifact (Join-Path $folder.FullName $name)
    if(!$expected -or $expected.bytes -ne $actual.bytes -or $expected.sha256 -ne $actual.sha256){throw 'Backup artifact checksum or length mismatch.'}
  }
  return $manifest
}
