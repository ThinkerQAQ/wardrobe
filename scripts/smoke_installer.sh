#!/usr/bin/env bash
set -euo pipefail
installer="${1:?installer path required}"
osname="${2:?target OS required}"
sandbox="$(mktemp -d "${RUNNER_TEMP:-/tmp}/wardrobe-ifw.XXXXXX")"
export HOME="$sandbox/home"
mkdir -p "$HOME" "$sandbox/mounted"
if [[ "$osname" == linux ]]; then export QT_QPA_PLATFORM=offscreen;fi
if [[ "$osname" == darwin ]]; then
  hdiutil attach -quiet -nobrowse -mountpoint "$sandbox/mounted" "$installer"
  trap 'hdiutil detach -quiet "$sandbox/mounted" || true' EXIT
  app="$(find "$sandbox/mounted" -maxdepth 2 -type d -name '*.app' | head -1)"
  test -n "$app"
  installer="$(find "$app/Contents/MacOS" -maxdepth 1 -type f | head -1)"
fi
target="$sandbox/installed with spaces"
mkdir -p "$target/Data/photos" "$target/Private"
printf 'keep-me\n' > "$target/Data/photos/retain.txt"
printf 'private\n' > "$target/Private/retain.txt"
"$installer" --root "$target" --accept-licenses --confirm-command --auto-answer OverwriteTargetDirectory=Yes install
test -f "$target/browser-extension/manifest.json"
test -f "$target/README_ZH.md"
test -f "$target/Data/photos/retain.txt"
test -f "$target/Private/retain.txt"
if [[ "$osname" == windows ]]; then
  test -f "$target/wardrobe.exe"
  test -f "$target/wardrobe-host.exe"
else
  test -x "$target/wardrobe"
fi
test ! -e "$target/scripts/start-tunnel.sh"
test ! -e "$target/scripts/Start-Wardrobe-Tunnel.ps1"
if [[ "$osname" == windows ]]; then
  powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/smoke_native_windows.ps1 -InstallDir "$(cygpath -w "$target")"
fi
# First-run acceptance is separate from the installer: config and SQLite are created
# on demand, without a Tunnel or browser extension.
if [[ "$osname" != windows ]]; then
  "$target/wardrobe" -open=false >"$sandbox/wardrobe.log" 2>&1 &
  server_pid=$!
  healthy=0
  for i in $(seq 1 50); do
    if curl -fsS --max-time 2 http://127.0.0.1:17936/api/health 2>/dev/null | grep -q '"version":"1.1.0"'; then
      healthy=1
      break
    fi
    sleep .2
  done
  test "$healthy" = 1 || { cat "$sandbox/wardrobe.log"; exit 1; }
  kill -0 "$server_pid"
  test -f "$target/Data/config.toml"
  kill "$server_pid"
  wait "$server_pid" || true
fi
if [[ "$osname" == darwin && -d "$target/Wardrobe-Maintenance.app" ]]; then
  maintenance="$(find "$target/Wardrobe-Maintenance.app/Contents/MacOS" -maxdepth 1 -type f | head -1)"
else
  maintenance="$target/Wardrobe-Maintenance"
  [[ "$osname" != windows ]] || maintenance+=".exe"
fi
test -f "$maintenance"
"$maintenance" --confirm-command remove com.thinkerqaq.wardrobe
test "$(cat "$target/Data/photos/retain.txt")" = keep-me
test "$(cat "$target/Private/retain.txt")" = private
echo "WARDROBE_IFW_NATIVE_INSTALL_UNINSTALL_DATA_PRESERVED $osname"
