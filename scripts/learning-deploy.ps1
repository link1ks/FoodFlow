param([ValidateSet('init','up','status','stop')][string]$Action='status')
$ErrorActionPreference='Stop'
Set-Location -LiteralPath (Split-Path $PSScriptRoot -Parent)
$config=Join-Path $PWD '.cache/learning.env'
if($Action -eq 'init'){
  New-Item -ItemType Directory -Force .cache | Out-Null
  # CreateNew refuses to overwrite credentials belonging to an existing volume.
  $stream=[IO.File]::Open($config,[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None)
  $stream.Dispose()
  if($IsWindows){
    $identity=[Security.Principal.WindowsIdentity]::GetCurrent().User
    $acl=[Security.AccessControl.FileSecurity]::new()
    $acl.SetOwner($identity)
    $acl.SetAccessRuleProtection($true,$false)
    $acl.SetAccessRule([Security.AccessControl.FileSystemAccessRule]::new($identity,'FullControl','Allow'))
    Set-Acl -LiteralPath $config -AclObject $acl
  }else{
    [IO.File]::SetUnixFileMode($config,[IO.UnixFileMode]::UserRead -bor [IO.UnixFileMode]::UserWrite)
  }
  $password=[Convert]::ToHexString([Security.Cryptography.RandomNumberGenerator]::GetBytes(24))
  $secret=[Convert]::ToHexString([Security.Cryptography.RandomNumberGenerator]::GetBytes(48))
  [IO.File]::WriteAllText($config,"POSTGRES_PASSWORD=$password`nJWT_SECRET=$secret`n")
  Write-Output 'Created protected learning configuration; no existing .env or data changed.'
  exit 0
}
if(!(Test-Path -LiteralPath $config)){throw 'Run scripts/learning-deploy.ps1 init first.'}
$values=@{}
foreach($line in [IO.File]::ReadAllLines($config)){
  if($line -match '^(POSTGRES_PASSWORD|JWT_SECRET)=([A-Fa-f0-9]{48,})$'){
    if($values.ContainsKey($Matches[1])){throw 'Duplicate learning configuration key.'}
    $values[$Matches[1]]=$Matches[2]
  }elseif($line.Trim() -ne ''){throw 'Learning configuration must contain only generated credentials.'}
}
if($values.Count -ne 2 -or $values.JWT_SECRET.Length -lt 64){throw 'Incomplete or weak learning configuration.'}
function dc {
  # Bound output: never print resolved compose config, environments or raw logs.
  & docker compose --env-file $config -f compose.learning.yaml @args
  if($LASTEXITCODE -ne 0){throw 'Learning Compose command failed.'}
}
$saved=@{}
try {
  # Shell environment must not silently override this isolated project's secrets.
  foreach($key in $values.Keys){
    $saved[$key]=[Environment]::GetEnvironmentVariable($key,'Process')
    [Environment]::SetEnvironmentVariable($key,$values[$key],'Process')
  }
  switch($Action){
    up {
      dc up --build -d --wait --wait-timeout 120
      $ready=$false
      for($attempt=0;$attempt -lt 30;$attempt++){
        try {
          $response=Invoke-WebRequest http://127.0.0.1:17173/health/ready -TimeoutSec 2
          if($response.StatusCode -eq 200){$ready=$true;break}
        }catch{}
        Start-Sleep -Milliseconds 500
      }
      if(!$ready){throw 'Learning gateway is not ready before the bounded deadline.'}
      Write-Output 'Learning deployment ready: http://127.0.0.1:17173 (separate accounts/data).'
    }
    status { dc ps }
    stop { dc stop }
  }
}finally{
  foreach($key in $saved.Keys){
    if($null -eq $saved[$key]){Remove-Item -LiteralPath ("Env:"+$key) -ErrorAction SilentlyContinue}
    else{[Environment]::SetEnvironmentVariable($key,$saved[$key],'Process')}
  }
}
