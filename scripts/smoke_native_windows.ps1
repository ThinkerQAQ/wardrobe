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
      if ($health.app -eq 'wardrobe' -and $health.version -eq '1.1.2' -and $health.status -eq 'ok') {
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
  $stdioInfo=[System.Diagnostics.ProcessStartInfo]::new()
  $stdioInfo.FileName=$bin
  $stdioInfo.Arguments='--mcp-stdio --config "' + (Join-Path $InstallDir 'Data/config.toml') + '"'
  $stdioInfo.WorkingDirectory=$InstallDir
  $stdioInfo.UseShellExecute=$false
  $stdioInfo.CreateNoWindow=$true
  $stdioInfo.RedirectStandardInput=$true
  $stdioInfo.RedirectStandardOutput=$true
  $stdioInfo.RedirectStandardError=$true
  $stdio=[System.Diagnostics.Process]::Start($stdioInfo)
  try {
    $meta='"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"wardrobe-native-smoke","version":"1"},"io.modelcontextprotocol/clientCapabilities":{}}'
    foreach($pair in @(@(1,'server/discover'),@(2,'tools/list'))) {
      $request='{"jsonrpc":"2.0","id":' + $pair[0] + ',"method":"' + $pair[1] + '","params":{' + $meta + '}}'
      $stdio.StandardInput.WriteLine($request)
      $stdio.StandardInput.Flush()
      $task=$stdio.StandardOutput.ReadLineAsync()
      if (!$task.Wait(10000)) { throw ('Modern MCP request timed out: '+$pair[1]) }
      $result=$task.Result | ConvertFrom-Json
      if ($result.error) { throw ('Modern MCP error: '+$pair[1]) }
      if ($result.id -ne $pair[0]) { throw 'Modern MCP response ID mismatch' }
      if ($pair[0] -eq 1 -and @($result.result.supportedVersions) -notcontains '2026-07-28') {
        throw 'Modern MCP discovery unsupported'
      }
      if ($pair[0] -eq 2 -and @($result.result.tools).Count -ne 30) {
        throw 'Modern MCP tool discovery returned incorrect count'
      }
    }
    Write-Output 'WINDOWS_MCP_2026_DISCOVERY_OK'
  } finally {
    if ($stdio -and !$stdio.HasExited) { $stdio.Kill(); $stdio.WaitForExit(8000) | Out-Null }
    if ($stdio) { $stdio.Dispose() }
  }
  Write-Output 'WINDOWS_FIRST_RUN_HEALTH_OK'
} finally {
  if ($proc -and !$proc.HasExited) { $proc.Kill(); $proc.WaitForExit(8000) | Out-Null }
  if ($proc) { $proc.Dispose() }
}
