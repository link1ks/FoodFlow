param(
  [ValidateSet('check','update')][string]$Action='update',
  [switch]$Restart
)
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'update-common.ps1')
try {
  Invoke-FoodFlowUpdate -Root (Split-Path $PSScriptRoot -Parent) -Action $Action -Restart:$Restart
} catch {
  Write-Host ("Update failed: " + $_.Exception.Message) -ForegroundColor Red
  exit 1
}
