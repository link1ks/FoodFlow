# Runtime checks and fictional photo fixture against the fixed learning project.
$ErrorActionPreference='Stop'
Set-Location -LiteralPath (Split-Path $PSScriptRoot -Parent)
function dc {
  & docker compose --env-file .cache/learning.env -f compose.learning.yaml @args
  if($LASTEXITCODE -ne 0){throw 'Learning Compose check failed.'}
}
function Check($ok,$message){if(!$ok){throw $message}}
foreach($service in @('db','api','worker','web')){
  $id=(dc ps -q $service).Trim()
  Check (![string]::IsNullOrWhiteSpace($id)) "Missing learning service: $service"
  # Explicit field selection prevents authenticated environment values appearing.
  $project=(& docker inspect --format '{{index .Config.Labels "com.docker.compose.project"}}' $id).Trim()
  Check ($LASTEXITCODE -eq 0 -and $project -eq 'foodflow-learning') 'Refusing a foreign project.'
  $ports=(& docker inspect --format '{{json .HostConfig.PortBindings}}' $id) | ConvertFrom-Json -AsHashtable
  Check ($LASTEXITCODE -eq 0) 'Cannot inspect port bindings.'
  if($service -eq 'web'){
    Check ($ports.Count -eq 1 -and $ports['80/tcp'].Count -eq 1 -and $ports['80/tcp'][0].HostIp -eq '127.0.0.1' -and $ports['80/tcp'][0].HostPort -eq '17173') 'Gateway must bind only the fixed loopback port.'
  }else{Check ($ports.Count -eq 0) "Internal service publishes a host port: $service"}
  $opts=(& docker inspect --format '{{json .HostConfig.SecurityOpt}}' $id) | ConvertFrom-Json
  Check ($LASTEXITCODE -eq 0 -and ('no-new-privileges:true' -in $opts -or 'no-new-privileges' -in $opts)) 'Privilege escalation guard missing.'
  if($service -ne 'db'){
    $user=(& docker inspect --format '{{.Config.User}}' $id).Trim()
    Check ($LASTEXITCODE -eq 0 -and $user -match '^(10001:10001|101:101)$') 'Application must run as a non-root user.'
    $readonly=(& docker inspect --format '{{.HostConfig.ReadonlyRootfs}}' $id).Trim()
    Check ($LASTEXITCODE -eq 0 -and $readonly -eq 'true') 'Application root filesystem must be read-only.'
    $caps=(& docker inspect --format '{{json .HostConfig.CapDrop}}' $id) | ConvertFrom-Json
    Check ($LASTEXITCODE -eq 0 -and 'ALL' -in $caps) 'Application must drop Linux capabilities.'
  }
}
foreach($path in @('/','/health/ready','/api/me','/assets/does-not-exist.js','/metrics')){
  $response=Invoke-WebRequest ("http://127.0.0.1:17173"+$path) -SkipHttpErrorCheck -TimeoutSec 10
  Check ($response.Headers['X-Content-Type-Options'] -contains 'nosniff') "Missing nosniff: $path"
  Check ($response.Headers['X-Frame-Options'] -contains 'DENY') "Missing frame guard: $path"
  Check ($response.Headers['Content-Security-Policy'] -like "*script-src 'self';*") "Missing executable script restriction: $path"
  if($path -eq '/health/ready'){Check ($response.StatusCode -eq 200) 'Gateway readiness failed.'}
  if($path -eq '/api/me'){
    Check ($response.StatusCode -eq 401) 'Anonymous account request must fail.'
    Check ($response.Headers['Cache-Control'] -contains 'no-store') 'Account response must not be cached.'
  }
  if($path -eq '/metrics'){Check ($response.StatusCode -eq 404) 'Metrics exposed through gateway.'}
  if($path -eq '/assets/does-not-exist.js'){Check ($response.StatusCode -eq 404) 'Missing asset must not return SPA HTML.'}
}
# Each gateway request must use the transport peer, never caller supplied identity.
$id=(dc ps -q web).Trim()
$nginx=& docker exec $id nginx -T 2>$null
Check ($LASTEXITCODE -eq 0 -and ($nginx -join "`n") -match 'proxy_set_header X-Real-IP \$remote_addr;') 'Gateway does not overwrite client identity.'
# Prove the non-root API can write the explicit volume and return identical bytes.
# Omit the optional threshold instead of sending the invalid quantity "0".
$base='http://127.0.0.1:17173/api'
$fixture='learning-security-'+[guid]::NewGuid().ToString('N')+'@example.test'
$account=Invoke-RestMethod -Method Post -Uri "$base/register" -ContentType application/json -Body (@{email=$fixture;password='Learning-Security-Fixture-2026!';name='隔离部署验收'}|ConvertTo-Json)
$headers=@{Authorization='Bearer '+$account.token}
$household=Invoke-RestMethod -Method Post -Uri "$base/households" -Headers $headers -ContentType application/json -Body '{"name":"隔离部署验收","servings":2}'
$ingredient=Invoke-RestMethod -Method Post -Uri "$base/households/$($household.id)/ingredients" -Headers $headers -ContentType application/json -Body '{"name":"番茄","category":"蔬菜","unit":"g"}'
$photo=Get-ChildItem -LiteralPath web/public/ingredient-photos -Filter '*.webp' | Select-Object -First 1
Check ($null -ne $photo) 'Missing local photo fixture.'
$null=Invoke-RestMethod -Method Post -Uri "$base/households/$($household.id)/ingredients/$($ingredient.id)/image" -Headers $headers -Form @{image=$photo}
$response=Invoke-WebRequest -Uri "$base/households/$($household.id)/ingredients/$($ingredient.id)/image" -Headers $headers
[byte[]]$original=[IO.File]::ReadAllBytes($photo.FullName)
[byte[]]$received=$response.Content
Check ($received.Length -gt 0) 'Missing image response bytes.'
Check ([Convert]::ToHexString([Security.Cryptography.SHA256]::HashData($original)) -eq [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData($received))) 'Stored photo bytes differ.'
$report=@{recorded_at=[DateTime]::UtcNow.ToString('o');project='foodflow-learning';passed=$true;checks=@('loopback-only gateway','no internal host ports','non-root readonly applications','capability and escalation guards','security headers on success/errors','no account caching','metrics blocked','gateway overwrites client identity','non-root household photo write/read SHA256');limitations=@('local Docker transport; no cloud or HTTPS acceptance','deterministic integrations only','not a backup/restore or real household trial')}
New-Item -ItemType Directory -Force .cache/harness | Out-Null
$report|ConvertTo-Json -Depth 5|Set-Content -LiteralPath .cache/harness/learning-security.json -Encoding utf8NoBOM
Write-Output 'PASS: isolated learning deployment security checks; redacted evidence saved.'
