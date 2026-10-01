$ErrorActionPreference='Stop'
Set-Location -LiteralPath (Split-Path $PSScriptRoot -Parent)
$digestTaskValue=(& go run ./cmd/harness -mode fingerprint).Trim()
if($LASTEXITCODE -ne 0 -or $digestTaskValue -notmatch '^[a-f0-9]{64}$'){throw 'Cannot compute acceptance source digest'}
$env:FOODFLOW_SOURCE_DIGEST=$digestTaskValue
& docker compose --env-file .cache/acceptance.env -f compose.acceptance.yaml build
if($LASTEXITCODE -ne 0){throw 'Acceptance image build failed'}
$images=@{}
foreach($name in @('api','web')){
 $tag="foodflow-${name}:acceptance"
 $id=(& docker image inspect --format '{{.Id}}' $tag).Trim()
 if($LASTEXITCODE -ne 0){throw 'Cannot read built image identity'}
 $label=(& docker image inspect --format '{{index .Config.Labels "io.foodflow.source-digest"}}' $tag).Trim()
 if($LASTEXITCODE -ne 0 -or $label -ne $digestTaskValue -or $id -notmatch '^sha256:[a-f0-9]{64}$'){throw 'Built image label does not match source digest'}
 $images[$name]=@{id=$id;source_digest=$label}
}
$revision=(& git rev-parse HEAD).Trim()
New-Item -ItemType Directory -Force .cache/harness | Out-Null
@{source_digest=$digestTaskValue;revision=$revision;images=$images}|ConvertTo-Json -Depth 5|Set-Content -LiteralPath .cache/harness/acceptance-images.json -Encoding utf8NoBOM
Write-Output 'Recorded acceptance source digest and immutable API/web image IDs'
