param(
 [string]$InstallDir = (Join-Path $env:LOCALAPPDATA 'Wardrobe'),
 [string]$WardrobeBinary = '',
 [switch]$NoTunnel,
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
function Install-TunnelClient {
  $exe = Join-Path $binDir 'tunnel-client.exe'
  if (Test-Path -LiteralPath $exe -PathType Leaf) { Say 'Reusing installed tunnel-client';return $exe }
  Say 'Downloading official openai/tunnel-client...'
  $release = Invoke-RestMethod 'https://api.github.com/repos/openai/tunnel-client/releases/latest' -Headers @{'User-Agent'='Wardrobe-Setup'}
  $architecture = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64' -or $env:PROCESSOR_ARCHITEW6432 -eq 'ARM64') { 'arm64' } else { 'amd64' }
  $asset = $release.assets | Where-Object { $_.name -match ('^tunnel-client-v[\d.]+-windows-'+$architecture+'\.zip$') } | Select-Object -First 1
  $checksum = $release.assets | Where-Object { $_.name -eq 'SHA256SUMS.txt' } | Select-Object -First 1
  if (!$asset -or !$checksum) { throw 'Official tunnel-client release archive/checksums missing' }
  $scratch = Scratch
  $archive = Join-Path $scratch $asset.name
  $sums = Join-Path $scratch 'tunnel-client.SHA256SUMS.txt'
  Download-File $asset.browser_download_url $archive
  Download-File $checksum.browser_download_url $sums
  Verify-Release $archive $sums
  $where = Join-Path $scratch 'tunnel-client'
  Expand-Archive -LiteralPath $archive -DestinationPath $where -Force
  $found = Get-ChildItem -LiteralPath $where -Recurse -Filter 'tunnel-client.exe' -File | Select-Object -First 1
  if (!$found) { throw 'Official tunnel-client archive lacks the CLI executable' }
  New-Item -ItemType Directory -Path $binDir -Force | Out-Null
  Copy-Item -LiteralPath $found.FullName -Destination $exe -Force
  return $exe
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
  $config = Join-Path $dataDir 'config.toml'
  if (!(Test-Path $config)) {
    $example = Join-Path $packageRoot 'config.example.toml'
    if (Test-Path $example) { Copy-Item $example $config }
    else { Set-Content -LiteralPath $config -Encoding UTF8 -Value '[server]','address = "127.0.0.1:17936"','open_browser = false','','[storage]','data_dir = "."' }
  }
  foreach ($s in @('Start-Wardrobe-Local.ps1','Start-Wardrobe-Tunnel.ps1')) {
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
  Say 'Local installation: OK'
  if (!$NoStart) {
    & (Join-Path $InstallDir 'scripts\Start-Wardrobe-Local.ps1')
    if (!(Check-Local)) { Start-Sleep -Seconds 2 }
    if (!(Check-Local)) { throw 'Wardrobe local health endpoint did not respond on 127.0.0.1:17936' }
    StartupShortcut 'Wardrobe Local' (Join-Path $InstallDir 'scripts\Start-Wardrobe-Local.ps1')
    Say 'Web UI: http://127.0.0.1:17936'
  }

  $useTunnel = !$NoTunnel
  if (!$NoTunnel -and !$NonInteractive) {
    $answer = Read-Host 'Connect this private wardrobe to ChatGPT via Secure MCP Tunnel? [Y/n]'
    $useTunnel = ($answer -eq '' -or $answer -match '^[Yy]')
  } elseif ($NonInteractive) { $useTunnel = $false }
  if ($useTunnel) {
    $tunnelExe = Install-TunnelClient
    $idFile = Join-Path $private 'tunnel-id.txt'
    $id = ''
    if (Test-Path $idFile) { $id = (Get-Content $idFile -Raw).Trim() }
    if (!$id) {
      Say 'A personal OpenAI Platform Tunnel ID is required. This cannot be created without your own permissions.'
      Start-Process 'https://platform.openai.com/settings/organization/tunnels'
      $id = Read-Host 'Paste your own tunnel_id (not an API key)'
    }
    if ($id -notmatch '^tunnel_[a-z0-9]{32}$') { throw 'Invalid Tunnel ID. Create your own in OpenAI Platform and rerun setup.' }
    [IO.File]::WriteAllText($idFile,$id)
    $secretFile = Join-Path $private 'TunnelKey.dpapi'
    if (!(Test-Path -LiteralPath $secretFile)) {
      Say 'Create a restricted runtime API Key in OpenAI Platform. Never use an admin key for the runtime.'
      Start-Process 'https://platform.openai.com/settings/organization/api-keys'
      $secret = Read-Host 'Paste runtime API key (hidden, encrypted locally)' -AsSecureString
      if ($secret.Length -lt 12) { throw 'No valid key entered' }
      $encrypted = ConvertFrom-SecureString -SecureString $secret
      [IO.File]::WriteAllText($secretFile,$encrypted)
      $secret.Dispose()
    }
    $secureValue = (Get-Content $secretFile -Raw).Trim() | ConvertTo-SecureString
    $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secureValue)
    try { $env:CONTROL_PLANE_API_KEY = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr) }
    finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr) }
    $command = '"'+$targetExe+'" -mcp-stdio -config "'+$config+'"'
    & $tunnelExe init --force --sample sample_mcp_stdio_local --profile wardrobe --profile-dir $profileDir --tunnel-id $id --mcp-command $command --health-listen-addr '127.0.0.1:0' --control-plane-api-key-ref env:CONTROL_PLANE_API_KEY
    if ($LASTEXITCODE -ne 0) { throw 'tunnel-client init failed' }
    & $tunnelExe doctor --profile wardrobe --profile-dir $profileDir --explain
    if ($LASTEXITCODE -ne 0) { throw 'Tunnel doctor failed. Check Tunnel association, runtime key and permissions, then rerun setup.' }
    Remove-Item Env:\CONTROL_PLANE_API_KEY -ErrorAction SilentlyContinue
    if (!$NoStart) {
      $runner = Join-Path $InstallDir 'scripts\Start-Wardrobe-Tunnel.ps1'
      StartupShortcut 'Wardrobe MCP Tunnel' $runner
      Start-Process -FilePath (Join-Path $env:WINDIR 'System32\WindowsPowerShell\v1.0\powershell.exe') -ArgumentList @('-NoProfile','-ExecutionPolicy','Bypass','-WindowStyle','Hidden','-File',('"'+$runner+'"')) -WindowStyle Hidden
    }
    Say 'Tunnel configured using your own ID; runtime key stored with Windows DPAPI.'
    Say 'Finish in ChatGPT: Plugins > + > Add custom MCP server > Tunnel. Select the same tunnel and approve the connection.'
    Start-Process 'https://chatgpt.com/'
  } else { Say 'ChatGPT Tunnel skipped. You can rerun this installer to connect later.' }
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
