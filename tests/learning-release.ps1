param([Parameter(Mandatory)][string]$Release)
$ErrorActionPreference='Stop'
Set-Location -LiteralPath (Split-Path $PSScriptRoot -Parent)
. (Join-Path $PSScriptRoot '../scripts/recovery-common.ps1')
. (Join-Path $PSScriptRoot '../scripts/release-common.ps1')
$m=Assert-ReleaseBundle $Release
$script:RecoveryProcessEnv=Read-RecoveryCredentials
function DBID {(Invoke-RecoveryDocker -Arguments @('ps','-q','--no-trunc','--filter','label=com.docker.compose.project=foodflow-learning','--filter','label=com.docker.compose.service=db') -Label 'learning DB').Trim()}
function ReadState {
  # Test-only read: immutable inventory/ledger/photo facts, never raw authenticated logs.
  $value=Invoke-RecoveryDocker -Arguments @('compose','--env-file','.cache/learning.env','-f','compose.learning.yaml','run','--rm','--no-deps','-T','api','/app/recovery','-mode','snapshot') -Label 'read-only release state'
  return $value|ConvertFrom-Json -AsHashtable
}
$before=ReadState;$db=DBID
& pwsh -NoProfile -File scripts/learning-release.ps1 check -Release $Release
if($LASTEXITCODE -ne 0){throw 'Read-only release check failed.'}
& pwsh -NoProfile -File scripts/learning-release.ps1 switch -Release $Release -ConfirmSwitch
if($LASTEXITCODE -ne 0){throw 'Same-version recreate rehearsal failed.'}
$after=ReadState
# Read-only snapshot includes transient rate-limit/job timestamps; compare the business facts affected by switching.
foreach($name in @('ingredients','batches','stock_ledger','stock_ledger_snapshots','stock_outbox','idempotency','batch_purchase_costs')){
  $a=$before.tables|Where-Object {$_.name -eq $name};$b=$after.tables|Where-Object {$_.name -eq $name}
  if(!$a -or !$b -or ($a|ConvertTo-Json -Compress) -cne ($b|ConvertTo-Json -Compress)){throw 'Business facts changed during release rehearsal.'}
}
if((DBID) -cne $db -or ($before.images|ConvertTo-Json -Compress) -cne ($after.images|ConvertTo-Json -Compress)){throw 'Database container or photos changed.'}
& pwsh -NoProfile -File tests/learning-security.ps1
if($LASTEXITCODE -ne 0){throw 'Security boundary changed after switching.'}
New-Item -ItemType Directory -Force .cache/harness | Out-Null
@{passed=$true;recorded_at=[DateTime]::UtcNow.ToString('o');source_digest=$m.source_digest;revision=$m.revision;checks=@('artifact and retained image binding','same-schema same-version container recreation','paired pre-switch backup','database container and migration history retained','inventory/ledger/outbox/request/photo facts retained','learning security after switch');limitations=@('same version rehearsal, not an older binary compatibility test','local retained images, no offline image export or public deployment')}|ConvertTo-Json -Depth 8|Set-Content -LiteralPath .cache/harness/learning-release.json -Encoding utf8NoBOM
Write-Output 'PASS: local release recreate rehearsal; redacted evidence saved.'
