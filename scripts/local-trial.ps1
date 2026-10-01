param([ValidateSet('up','status')][string]$Action='status',[switch]$UseDeepSeek)
$ErrorActionPreference='Stop'
Set-Location -LiteralPath (Split-Path $PSScriptRoot -Parent)
# Explicit Compose values also work when the host shell treats empty env as unset.
$composeFiles=@('-f','compose.yaml','-f','compose.trial.yaml')
if($UseDeepSeek){$composeFiles+=@('-f','compose.trial-deepseek.yaml')}
function dc {& docker compose @composeFiles @args;if($LASTEXITCODE -ne 0){throw 'Local trial Compose command failed'}}
if($UseDeepSeek -and $Action -eq 'up'){
 $trialConfig=dc config --format json | ConvertFrom-Json
 foreach($name in @('api','worker')){
  $provider=$trialConfig.services.$name.environment
  $providerUri=[uri]$provider.MODEL_ENDPOINT
  if($providerUri.Scheme -ne 'https' -or $providerUri.Host -ne 'api.deepseek.com' -or $providerUri.UserInfo -or $providerUri.Query -or $providerUri.Fragment -or $providerUri.AbsolutePath.TrimEnd('/') -notin @('','/v1') -or $providerUri.Port -ne 443 -or $provider.MODEL_NAME -ne 'deepseek-flash' -or !$provider.MODEL_API_KEY){throw 'Expected saved official HTTPS DeepSeek Flash configuration'}
 }
 $trialConfig=$null
}
if($Action -eq 'up'){dc up -d --build --wait --wait-timeout 180}
dc ps
$ready=Invoke-WebRequest -Uri http://localhost:5173/health/ready -TimeoutSec 10
if($ready.StatusCode -ne 200){throw 'Local trial is not ready'}
dc exec -T api sh -c 'if [ -n "$MODEL_ENDPOINT" ] && [ -n "$MODEL_API_KEY" ] && [ -n "$MODEL_NAME" ]; then echo "Text model: enabled (billable)"; else echo "Text model: deterministic demo"; fi; if [ -n "$VISION_MODEL_ENDPOINT" ] && [ -n "$VISION_MODEL_API_KEY" ] && [ -n "$VISION_MODEL_NAME" ]; then echo "Vision model: enabled (billable)"; else echo "Vision model: disabled"; fi'
Write-Output 'FoodFlow family trial: http://localhost:5173 (local storage)'
