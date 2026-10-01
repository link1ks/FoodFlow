param([ValidateSet('up','status')][string]$Action='status')
$ErrorActionPreference='Stop'
Set-Location -LiteralPath (Split-Path $PSScriptRoot -Parent)
# Explicit Compose values also work when the host shell treats empty env as unset.
function dc {& docker compose -f compose.yaml -f compose.trial.yaml @args;if($LASTEXITCODE -ne 0){throw 'Local trial Compose command failed'}}
if($Action -eq 'up'){dc up -d --build --wait --wait-timeout 180}
dc ps
$ready=Invoke-WebRequest -Uri http://localhost:5173/health/ready -TimeoutSec 10
if($ready.StatusCode -ne 200){throw 'Local trial is not ready'}
Write-Output 'FoodFlow family trial: http://localhost:5173 (deterministic demo, local storage)'
