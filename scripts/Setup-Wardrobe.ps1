param(
 [string]$InstallDir = (Join-Path $env:LOCALAPPDATA 'Wardrobe'),
 [string]$WardrobeBinary = '',
 [string]$ExtensionId = '',
 [switch]$NoStart,
 [switch]$NoAutostart,
 [switch]$NonInteractive
)
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$packageRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$InstallDir = [IO.Path]::GetFullPath($InstallDir)
$dataDir = Join-Path $InstallDir 'Data'
$private = Join-Path $InstallDir 'Private'
$profileDir = Join-Path $private 'profiles'
$binDir = Join-Path $InstallDir 'bin'
$sourceExe = Join-Path $packageRoot 'wardrobe.exe'
$script:temp = $null
function Say([string]$s) { Write-Host ("[Wardrobe] "+$s) }
function Download-File([string]$url,[string]$dest) {
  Invoke-WebRequest -Uri $url -OutFile $dest -UseBasicParsing -Headers @{'User-Agent'='Wardrobe-Setup'} -TimeoutSec 120
}
function Verify-Release([string]$assetPath,[string]$checksPath) {
  $name = [IO.Path]::GetFileName($assetPath)
  $lines = [IO.File]::ReadAllLines($checksPath)
  $match = $lines | Where-Object { $_ -match ('(?i)^[0-9a-f]{64}\s+\*?'+[regex]::Escape($name)+'$') } | Select-Object -First 1
  if (!$match) { throw "Checksum for $name not found in official release" }
  $expected = ($match -split '\s+')[0].ToLowerInvariant()
  $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $assetPath).Hash.ToLowerInvariant()
  if ($expected -ne $actual) { throw "SHA256 mismatch for $name" }
}
function Scratch {
  if (!$script:temp) { $script:temp = Join-Path ([IO.Path]::GetTempPath()) ("WardrobeSetup-"+[guid]::NewGuid().ToString('N'));New-Item -ItemType Directory -Path $script:temp -Force | Out-Null }
  return $script:temp
}
function Latest-Bundle {
  Say 'Downloading the official Wardrobe release (Go/npm not required)...'
  $scratch = Scratch
  $arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64' -or $env:PROCESSOR_ARCHITEW6432 -eq 'ARM64') { 'arm64' } else { 'amd64' }
  $name = 'wardrobe-windows-'+$arch+'.zip'
  $archive = Join-Path $scratch $name
  Download-File ('https://github.com/ThinkerQAQ/wardrobe/releases/latest/download/'+$name) $archive
  $checks = Join-Path $scratch 'SHA256SUMS.txt'
  Download-File 'https://github.com/ThinkerQAQ/wardrobe/releases/latest/download/SHA256SUMS.txt' $checks
  Verify-Release $archive $checks
  $dest = Join-Path $scratch 'wardrobe'
  Expand-Archive -LiteralPath $archive -DestinationPath $dest -Force
  $exe = Get-ChildItem -LiteralPath $dest -Recurse -Filter 'wardrobe.exe' -File | Select-Object -First 1
  if (!$exe) { throw 'Downloaded package is missing wardrobe.exe' }
  $script:packageRoot = $exe.Directory.FullName
  return $exe.FullName
}
function StartupShortcut([string]$name,[string]$script) {
  if ($NoAutostart -or $NoStart) { return }
  $folder = [Environment]::GetFolderPath('Startup')
  if (!$folder) { throw 'Windows Startup folder not found' }
  $shell = New-Object -ComObject WScript.Shell
  $link = $shell.CreateShortcut((Join-Path $folder ($name+'.lnk')))
  $link.TargetPath = (Join-Path $env:WINDIR 'System32\WindowsPowerShell\v1.0\powershell.exe')
  $link.Arguments = '-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File "'+$script+'"'
  $link.WorkingDirectory = $InstallDir
  $brandIcon = Join-Path $InstallDir 'brand\wardrobe.ico'
  if (Test-Path -LiteralPath $brandIcon) { $link.IconLocation = $brandIcon }
  $link.WindowStyle = 7
  $link.Save()
}
function Check-Local {
  try { $health = Invoke-RestMethod 'http://127.0.0.1:17936/api/health' -TimeoutSec 3
    return $health.app -eq 'wardrobe'
  } catch { return $false }
}

try {
  Say 'Preparing per-user directory; existing SQLite, photos and ratings will be kept.'
  foreach ($folder in @($InstallDir,$dataDir,$private,$binDir,$profileDir,(Join-Path $InstallDir 'scripts'))) {
    New-Item -ItemType Directory -Force -Path $folder | Out-Null
  }
  if ($WardrobeBinary) { $sourceExe = [IO.Path]::GetFullPath($WardrobeBinary) }
  elseif (!(Test-Path -LiteralPath $sourceExe -PathType Leaf)) { $sourceExe = Latest-Bundle }
  if (!(Test-Path -LiteralPath $sourceExe -PathType Leaf)) { throw "Wardrobe executable not found: $sourceExe" }
  $targetExe = Join-Path $InstallDir 'wardrobe.exe'
  if ([IO.Path]::GetFullPath($sourceExe) -ne [IO.Path]::GetFullPath($targetExe)) {
    Copy-Item -LiteralPath $sourceExe -Destination ($targetExe+'.new') -Force
    try { Move-Item -LiteralPath ($targetExe+'.new') -Destination $targetExe -Force }
    catch { Remove-Item -LiteralPath ($targetExe+'.new') -ErrorAction SilentlyContinue;throw 'Cannot replace Wardrobe while it is running; close the old Wardrobe instance and retry' }
  }
  # The GUI-subsystem Native Host must be upgraded alongside wardrobe.exe.
  $sourceHost = Join-Path $packageRoot 'wardrobe-host.exe'
  $targetHost = Join-Path $InstallDir 'wardrobe-host.exe'
  if (Test-Path -LiteralPath $sourceHost -PathType Leaf) {
    if ([IO.Path]::GetFullPath($sourceHost) -ne [IO.Path]::GetFullPath($targetHost)) {
      Copy-Item -LiteralPath $sourceHost -Destination $targetHost -Force
    }
  }
  # Rebind the old manifest without changing existing extension allowlists.
  & $targetExe --repair-native-host
  if ($LASTEXITCODE -ne 0) { throw 'Native Messaging repair failed' }
  $config = Join-Path $dataDir 'config.toml'
  if (!(Test-Path $config)) {
    $example = Join-Path $packageRoot 'config.example.toml'
    if (Test-Path $example) { Copy-Item $example $config }
    else { Set-Content -LiteralPath $config -Encoding UTF8 -Value '[server]','address = "127.0.0.1:17936"','open_browser = false','','[storage]','data_dir = "."' }
  }
  foreach ($s in @('Start-Wardrobe-Local.ps1','Open-Wardrobe.ps1','Guide-Wardrobe-Extension.ps1')) {
    $from = Join-Path $packageRoot ('scripts\'+$s)
    if (!(Test-Path $from)) { throw "Setup package is missing scripts\$s" }
    $to = Join-Path $InstallDir ('scripts\'+$s)
    if ($from -ne $to) { Copy-Item $from $to -Force }
  }
  # Public branding assets only. Preserve the complete existing Data tree.
  $brandSource = Join-Path $packageRoot 'brand'
  if (Test-Path -LiteralPath (Join-Path $brandSource 'wardrobe.ico')) {
    $brandTarget = Join-Path $InstallDir 'brand'
    New-Item -ItemType Directory -Force -Path $brandTarget | Out-Null
    foreach ($name in @('wardrobe.ico','wardrobe-mark.svg')) {
      $from = Join-Path $brandSource $name
      if (Test-Path -LiteralPath $from) { Copy-Item -LiteralPath $from -Destination (Join-Path $brandTarget $name) -Force }
    }
  }
  # Bundle the same reviewed side-panel UI next to the installed program.
  # This never loads an extension into Edge without the user's browser action.
  $extensionSource = Join-Path $packageRoot 'browser-extension'
  $extensionTarget = Join-Path $InstallDir 'browser-extension'
  if (Test-Path -LiteralPath (Join-Path $extensionSource 'app\index.html')) {
    if ([IO.Path]::GetFullPath($extensionSource) -ne [IO.Path]::GetFullPath($extensionTarget)) {
      New-Item -ItemType Directory -Force -Path $extensionTarget | Out-Null
      Copy-Item -Path (Join-Path $extensionSource '*') -Destination $extensionTarget -Recurse -Force
    }
    # Remove the obsolete orders-only popup from the previous version.
    foreach ($legacy in @('popup.html','popup.js')) {
      $obsolete = Join-Path $extensionTarget $legacy
      if (Test-Path -LiteralPath $obsolete) { Remove-Item -LiteralPath $obsolete -Force }
    }
    Say 'Wardrobe side panel files installed. In Edge > Extensions, load this unpacked folder once.'
  } else { Say 'Browser extension assets missing from this custom package; Web UI remains available.' }
  if ($ExtensionId) {
    if ($ExtensionId -cnotmatch '^[a-p]{32}$') { throw 'Invalid Edge/Chrome extension ID (expected 32 a-p letters)' }
    & $targetExe --register-native-host $ExtensionId
    if ($LASTEXITCODE -ne 0) { throw 'Native Messaging registration failed' }
    Say 'Native Messaging: enabled for extension '+$ExtensionId
  } else {
    Say 'To enable on-demand startup, rerun setup with -ExtensionId from Edge/Chrome extension details.'
  }
  Say 'Local installation: OK'
  if (!$NoStart) {
    & (Join-Path $InstallDir 'scripts\Start-Wardrobe-Local.ps1')
    if (!(Check-Local)) { Start-Sleep -Seconds 2 }
    if (!(Check-Local)) { throw 'Wardrobe local health endpoint did not respond on 127.0.0.1:17936' }
    StartupShortcut 'Wardrobe Local' (Join-Path $InstallDir 'scripts\Start-Wardrobe-Local.ps1')
    Say 'Web UI: http://127.0.0.1:17936'
  }

  Say 'Configure optional ChatGPT access in Wardrobe > MCP Connection.'
  if (!$NoStart -and !$NonInteractive) { Start-Process 'http://127.0.0.1:17936' }
  Say 'Setup finished. Check ChatGPT tool discovery with a read-only wardrobe search.'
  exit 0
} catch {
  Write-Host ('[Wardrobe] SETUP INCOMPLETE: '+$_.Exception.Message) -ForegroundColor Red
  Write-Host 'Local data has not been erased; rerun Setup-Wardrobe.cmd after resolving the issue.'
  exit 1
} finally {
  Remove-Item Env:\CONTROL_PLANE_API_KEY -ErrorAction SilentlyContinue
  if ($script:temp -and (Test-Path $script:temp)) { Remove-Item -LiteralPath $script:temp -Recurse -Force -ErrorAction SilentlyContinue }
}
