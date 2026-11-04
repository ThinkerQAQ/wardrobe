$ErrorActionPreference='Stop'
$root = Split-Path -Parent $PSScriptRoot
$ext = Join-Path $root 'browser-extension'
if (!(Test-Path (Join-Path $ext 'manifest.json'))) { throw 'Wardrobe extension missing' }
Start-Process explorer.exe -ArgumentList ('"' + $ext + '"')
Start-Process 'msedge.exe' -ArgumentList 'edge://extensions' -ErrorAction SilentlyContinue
