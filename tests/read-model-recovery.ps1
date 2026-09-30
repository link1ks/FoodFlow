$ErrorActionPreference='Stop'
Import-Module Microsoft.PowerShell.Utility -ErrorAction Stop
# Resolve the web cmdlet assembly explicitly on hosts with lazy module loading.
$null=Get-Command Invoke-RestMethod -ErrorAction Stop
. (Join-Path $PSScriptRoot 'read-model-helper.ps1')
function HttpError([int]$code){
  $response=[Net.Http.HttpResponseMessage]::new([Net.HttpStatusCode]$code)
  return [Microsoft.PowerShell.Commands.HttpResponseException]::new('Fictional test response',$response)
}
$script:calls=0
$result=Wait-ReadModel -Attempts 3 -DelayMs 1 -Read {
  $script:calls++
  if($script:calls -eq 1){throw (HttpError 503)}
  return @{quantity='10000'}
} -Ready {param($value) $value.quantity -eq '10000'}
if($script:calls -ne 2 -or $result.quantity -ne '10000'){throw 'Transient read recovery failed'}
$script:calls=0
try {
  Wait-ReadModel -Attempts 3 -DelayMs 1 -Read {$script:calls++;throw (HttpError 404)} -Ready {$true}
  throw 'Permanent error was swallowed'
}catch{
  if(!$_.Exception.Response -or [int]$_.Exception.Response.StatusCode -ne 404 -or $script:calls -ne 1){throw}
}
$script:calls=0
try{
  Wait-ReadModel -Attempts 2 -DelayMs 1 -Read {$script:calls++;return $false} -Ready {param($value) $value}
  throw 'Unconverged projection was accepted'
}catch{
  if($_.Exception.Message -ne 'Read projection did not converge before the bounded deadline' -or $script:calls -ne 2){throw}
}
Write-Output 'PASS: retry transient 503, reject permanent 404, enforce bounded convergence'
