#!/usr/bin/env bash
# No network, no real data, no personal browser profile. Fixture install only.
set -euo pipefail
root="$(mktemp -d)";trap 'rm -rf "$root"' EXIT
mkdir -p "$root/package/scripts" "$root/package/browser-extension/app" "$root/package/browser-extension/icons" "$root/package/brand"
cp scripts/setup.sh "$root/package/scripts/setup.sh"
cp scripts/start-local.sh scripts/start-tunnel.sh "$root/package/scripts/"
printf '#!/bin/sh\nexit 0\n' > "$root/package/wardrobe";chmod +x "$root/package/wardrobe"
printf 'fixture' > "$root/package/browser-extension/manifest.json"
printf 'fixture' > "$root/package/browser-extension/app/index.html"
printf 'obsolete' > "$root/package/browser-extension/popup.html"
printf 'brand' > "$root/package/brand/wardrobe.ico"
printf 'brand-svg' > "$root/package/brand/wardrobe-mark.svg"
printf 'icon' > "$root/package/browser-extension/icons/icon16.png"
bash "$root/package/scripts/setup.sh" --install-dir "$root/installed" --binary "$root/package/wardrobe" --no-start --no-tunnel --no-autostart --non-interactive >/dev/null
[[ "$(cat "$root/installed/browser-extension/app/index.html")" == 'fixture' ]]
[[ "$(cat "$root/installed/browser-extension/manifest.json")" == 'fixture' ]]
[[ ! -e "$root/installed/browser-extension/popup.html" ]]
[[ "$(cat "$root/installed/brand/wardrobe.ico")" == "brand" ]]
[[ "$(cat "$root/installed/browser-extension/icons/icon16.png")" == "icon" ]]
printf 'personal data' > "$root/installed/Data/user-record-sentinel.txt"
bash "$root/package/scripts/setup.sh" --install-dir "$root/installed" --binary "$root/package/wardrobe" --no-start --no-tunnel --no-autostart --non-interactive >/dev/null
[[ "$(cat "$root/installed/Data/user-record-sentinel.txt")" == 'personal data' ]]
printf 'Install preserves private data and includes shared extension UI: pass\n'
