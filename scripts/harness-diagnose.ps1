param([string]$Out='.cache/harness/diagnostics.json')
$ErrorActionPreference='Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
# This command targets the isolated acceptance stack only; never dumps env or payloads.
$config=Join-Path $PWD '.cache/acceptance.env'
$compose=@('compose','--env-file',$config,'-f','compose.acceptance.yaml','-f','compose.platform.yaml')
$checks=[Collections.Generic.List[object]]::new()
$samples=@{}
function Probe([string]$name,[scriptblock]$operation){
  try { $samples[$name]=& $operation; $checks.Add(@{name=$name;passed=$true}) }
  catch { $checks.Add(@{name=$name;passed=$false;error='Probe failed; inspect the named service locally. Raw errors and credentials are not exported.'}) }
}
function DC {
  $output=& docker @compose @args 2>$null
  if($LASTEXITCODE -ne 0){throw 'Compose probe failed'}
  return $output
}
Probe 'services' {
  $rows=DC ps --all --format json
  $allowed=@('db','api','worker','web','migrate','redis','kafka','kafka-init','kafka-volume-init','insights-db','insights','relay')
  foreach($line in $rows){
    if(!$line){continue}
    $entry=$line|ConvertFrom-Json
    foreach($item in @($entry)){
      if($item.Service -in $allowed){@{service=$item.Service;state=$item.State;health=$item.Health;exit_code=$item.ExitCode}}
    }
  }
}
Probe 'gateway_ready' { @{status=(Invoke-WebRequest 'http://127.0.0.1:15173/health/ready' -TimeoutSec 5).StatusCode} }
Probe 'api_ready' { @{status=(Invoke-WebRequest 'http://127.0.0.1:18080/health/ready' -TimeoutSec 5).StatusCode} }
Probe 'api_metrics' {
  $text=(Invoke-WebRequest 'http://127.0.0.1:18080/metrics' -TimeoutSec 5).Content
  foreach($line in ($text -split "`n")){
    if($line -match '^(foodflow_[a-z_]+) ([0-9]+)$'){@{name=$matches[1];value=$matches[2]}}
  }
}
Probe 'outbox' {
  $sql="SELECT json_build_object('pending',count(*) FILTER(WHERE published_at IS NULL),'oldest_pending_seconds',coalesce(extract(epoch FROM now()-min(created_at) FILTER(WHERE published_at IS NULL)),0),'publish_attempts',coalesce(sum(attempts),0)) FROM stock_outbox;"
  (DC exec -T db psql -U foodflow -d foodflow -At -c $sql) -join '' | ConvertFrom-Json
}
Probe 'jobs' {
  $sql="SELECT coalesce(json_agg(x),'[]') FROM (SELECT status,count(*) AS count,count(*) FILTER(WHERE status='running' AND lease_until<now()) AS expired_leases FROM jobs GROUP BY status ORDER BY status) x;"
  (DC exec -T db psql -U foodflow -d foodflow -At -c $sql) -join '' | ConvertFrom-Json
}
Probe 'projection' {
  $sql="SELECT json_build_object('facts',(SELECT count(*) FROM stock_facts),'rejected',(SELECT count(*) FROM rejected_events),'last_received_at',(SELECT max(received_at) FROM stock_facts));"
  (DC exec -T insights-db psql -U insights -d insights -At -c $sql) -join '' | ConvertFrom-Json
}
Probe 'kafka_lag' {
  $rows=DC exec -T kafka /opt/kafka/bin/kafka-consumer-groups.sh --bootstrap-server kafka:9092 --describe --group foodflow-insights-v1
  foreach($line in $rows){
    if($line -match '^\s*foodflow-insights-v1\s+foodflow\.stock\.v1\s+(\d+)\s+(\d+|-)\s+(\d+|-)\s+(\d+|-)'){
      @{partition=$matches[1];committed_offset=$matches[2];end_offset=$matches[3];lag=$matches[4]}
    }
  }
}
Probe 'structured_logs' {
  # Project an allowlist of known messages. Unknown or non-JSON logs are counted only.
  $known=@('http_request','outbox dispatch failed','Kafka fetch failed','projection apply or offset commit failed','insights HTTP failed')
  foreach($service in @('api','worker','relay','insights')){
    $kept=[Collections.Generic.List[object]]::new();$discarded=0
    foreach($line in (DC logs --no-log-prefix --no-color --tail 80 $service)){
      try{
        $row=$line|ConvertFrom-Json
        if($row.msg -notin $known){$discarded++;continue}
        $entry=@{time=$row.time;level=$row.level;message=$row.msg}
        if($row.msg -eq 'http_request'){
          # Only bounded route templates and numeric fields, never query/body/header/user IDs.
          if($row.path -match '^/[a-zA-Z0-9_/:.-]{0,180}$'){$entry.route=$row.path}
          if($row.method -in @('GET','POST','PATCH','DELETE','PUT','OPTIONS')){$entry.method=$row.method}
          if([string]$row.status -match '^\d{3}$'){$entry.status=[int]$row.status}
          if([string]$row.duration_ms -match '^\d+$'){$entry.duration_ms=[long]$row.duration_ms}
        }
        $kept.Add($entry)
      }catch{$discarded++}
    }
    @{service=$service;entries=$kept.ToArray();discarded_lines=$discarded}
  }
}
$report=@{version=1;recorded_at=[DateTimeOffset]::UtcNow.ToString('o');environment='isolated-local-acceptance';checks=$checks.ToArray();samples=$samples;scope='Point-in-time read-only probes, not production alerting or distributed tracing. No env, raw payloads or authenticated headers exported.'}
New-Item -ItemType Directory -Force (Split-Path $Out -Parent)|Out-Null
$report|ConvertTo-Json -Depth 12|Set-Content -LiteralPath $Out
Write-Output "Diagnostics written: $Out"
if(@($checks|Where-Object {!$_.passed}).Count){exit 1}
