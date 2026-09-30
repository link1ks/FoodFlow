function Wait-ReadModel {
  param([scriptblock]$Read,[scriptblock]$Ready,[int]$Attempts=90,[int]$DelayMs=500)
  # Only read-only probes are retried. Authentication/validation errors fail immediately.
  for($attempt=0;$attempt -lt $Attempts;$attempt++){
    try {
      $value=& $Read
      if(& $Ready $value){return $value}
    } catch {
      $response=$_.Exception.Response
      if(!$response -or [int]$response.StatusCode -notin @(502,503,504)){throw}
    }
    if($attempt+1 -lt $Attempts){Start-Sleep -Milliseconds $DelayMs}
  }
  throw 'Read projection did not converge before the bounded deadline'
}
