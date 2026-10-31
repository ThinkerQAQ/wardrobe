$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$private = Join-Path $root 'Private'
$bin = Join-Path $root 'bin\tunnel-client.exe'
$secret = Join-Path $private 'TunnelKey.dpapi'
$profileDir = Join-Path $private 'profiles'
if (!(Test-Path $bin) -or !(Test-Path $secret)) { exit 2 }
$mutex = New-Object System.Threading.Mutex($false, 'Local\WardrobeTunnelRuntime')
$owned = $false
try {
  try { $owned = $mutex.WaitOne(0) } catch [System.Threading.AbandonedMutexException] { $owned = $true }
  if (!$owned) { exit 0 } # a supervisor already runs under this Windows user
  $secure = (Get-Content -LiteralPath $secret -Raw).Trim() | ConvertTo-SecureString
  $bstr = [System.Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
  try { $env:CONTROL_PLANE_API_KEY = [System.Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr) }
  finally { [System.Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr) }
  while ($true) {
    & $bin run --profile 'wardrobe' --profile-dir $profileDir
    Start-Sleep -Seconds 8
  }
} finally {
  Remove-Item Env:\CONTROL_PLANE_API_KEY -ErrorAction SilentlyContinue
  if ($owned) { $mutex.ReleaseMutex() }
  $mutex.Dispose()
}
