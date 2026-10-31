param([switch]$Quiet)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$bin = Join-Path $root 'wardrobe.exe'
$config = Join-Path $root 'Data\config.toml'
if (!(Test-Path -LiteralPath $bin -PathType Leaf) -or !(Test-Path -LiteralPath $config -PathType Leaf)) { throw 'Wardrobe binary/config is missing. Rerun Setup-Wardrobe.cmd.' }
try {
  $health = Invoke-RestMethod -Uri 'http://127.0.0.1:17936/api/health' -TimeoutSec 2
  if ($health.app -eq 'wardrobe') { return }
} catch {}
Start-Process -FilePath $bin -ArgumentList @('-config', ('"'+$config+'"'), '-open=false') -WorkingDirectory $root -WindowStyle Hidden
if (!$Quiet) { Write-Host 'Wardrobe started locally: http://127.0.0.1:17936' }
