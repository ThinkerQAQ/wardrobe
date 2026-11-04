param(
  [Parameter(Mandatory=$true)][string]$InstallDir
)
$ErrorActionPreference='Stop'
$bin=Join-Path $InstallDir 'wardrobe.exe'
if (!(Test-Path $bin)) { throw 'Wardrobe executable not installed' }
$psi=New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName=$bin
$psi.Arguments='-open=false'
$psi.WorkingDirectory=$InstallDir
$psi.UseShellExecute=$false
$psi.CreateNoWindow=$true
$proc=[System.Diagnostics.Process]::Start($psi)
try {
  if (!$proc) { throw 'Wardrobe process did not start' }
  $healthy=$false
  for ($i=0; $i -lt 60; $i++) {
    if ($proc.HasExited) { throw "Wardrobe process exited: $($proc.ExitCode)" }
    try {
      $health=Invoke-RestMethod -Uri 'http://127.0.0.1:17936/api/health' -TimeoutSec 2
      if ($health.app -eq 'wardrobe' -and $health.version -eq '1.1.1' -and $health.status -eq 'ok') {
        $healthy=$true
        break
      }
    } catch {}
    Start-Sleep -Milliseconds 250
  }
  if (!$healthy) { throw 'Wardrobe health verification failed after native install' }
  if (!(Test-Path (Join-Path $InstallDir 'Data/config.toml'))) {
    throw 'First run did not initialize Data/config.toml'
  }
  Write-Output 'WINDOWS_FIRST_RUN_HEALTH_OK'
} finally {
  if ($proc -and !$proc.HasExited) { $proc.Kill(); $proc.WaitForExit(8000) | Out-Null }
  if ($proc) { $proc.Dispose() }
}
