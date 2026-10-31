$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
# The Go binary reads TOML, opens the configured URL and owns validation.
Start-Process -FilePath (Join-Path $root "wardrobe.exe") -WorkingDirectory $root -WindowStyle Hidden
