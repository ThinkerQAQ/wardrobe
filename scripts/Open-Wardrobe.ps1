$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$uri = 'http://127.0.0.1:17936'
function IsReady {
  try {
    $h = Invoke-RestMethod -Uri ($uri+'/api/health') -TimeoutSec 2
    return $h.app -eq 'wardrobe' -and $h.status -eq 'ok'
  } catch { return $false }
}
if (!(IsReady)) {
  $exe = Join-Path $root 'wardrobe.exe'
  if (!(Test-Path -LiteralPath $exe)) { throw 'Wardrobe executable missing' }
  Start-Process -FilePath $exe -ArgumentList '-open=false' -WindowStyle Hidden -WorkingDirectory $root
  for ($i=0; $i -lt 48 -and !(IsReady); $i++) { Start-Sleep -Milliseconds 250 }
  if (!(IsReady)) { throw 'Wardrobe service failed health verification (port 17936)' }
}
Start-Process $uri
