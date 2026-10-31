#!/usr/bin/env bash
set -euo pipefail
umask 077
SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEST="$HOME/.local/share/wardrobe"
BINARY=""
NO_TUNNEL=0; NO_START=0; NO_AUTOSTART=0; NONINTERACTIVE=0
while (($#)); do
 case "$1" in
 --install-dir) DEST="$2";shift 2;; --binary) BINARY="$2";shift 2;;
 --no-tunnel) NO_TUNNEL=1;shift;; --no-start) NO_START=1;shift;;
 --no-autostart) NO_AUTOSTART=1;shift;; --non-interactive) NONINTERACTIVE=1;shift;;
 --help|-h) echo 'Usage: bash scripts/setup.sh [--install-dir DIR] [--binary FILE] [--no-tunnel] [--no-start] [--no-autostart] [--non-interactive]';exit 0;;
 *) echo "Unknown option: $1" >&2;exit 2;; esac
done
say(){ echo "[Wardrobe] $*"; }
fail(){ echo "[Wardrobe] INCOMPLETE: $*" >&2;exit 1; }
for tool in curl tar;do command -v "$tool" >/dev/null || fail "$tool required";done
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in x86_64|amd64) ARCH=amd64;;arm64|aarch64) ARCH=arm64;;*)fail "Unsupported CPU: $ARCH";;esac
[[ "$OS" == linux || "$OS" == darwin ]] || fail 'Use Setup-Wardrobe.cmd for Windows'
if [[ "$OS" == linux ]];then command -v sha256sum >/dev/null || fail 'sha256sum required'
else command -v shasum >/dev/null || fail 'shasum required';fi
DEST="$(mkdir -p "$DEST" && cd "$DEST" && pwd -P)"
[[ "$DEST" != / && "$DEST" != "$HOME" ]] || fail 'Unsafe install directory'
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
download(){ curl -fL --retry 2 --connect-timeout 15 --max-time 180 -sS -o "$2" "$1"; }
checksha(){
 local archive="$1" sums="$2" want got
 want="$(awk -v f="$(basename "$archive")" '$2==f || $2=="*"f{print $1;exit}' "$sums")"
 [[ "$want" =~ ^[a-fA-F0-9]{64}$ ]] || fail 'Release SHA256 checksum missing'
 if [[ "$OS" == darwin ]];then got="$(shasum -a 256 "$archive" | awk '{print $1}')"
 else got="$(sha256sum "$archive" | awk '{print $1}')";fi
 [[ "$got" == "$want" ]] || fail 'Release SHA256 verification failed'
}
openurl(){
 ((NONINTERACTIVE))&&return
 if [[ "$OS" == darwin ]];then open "$1" >/dev/null 2>&1 || true
 else command -v xdg-open >/dev/null && xdg-open "$1" >/dev/null 2>&1 || true;fi
}
if [[ -z "$BINARY" ]];then
 if [[ -x "$SRC/wardrobe" ]];then BINARY="$SRC/wardrobe"
 else
  say 'Downloading official Wardrobe release (Go/npm not required)'
  name="wardrobe-$OS-$ARCH.tar.gz"
  download "https://github.com/ThinkerQAQ/wardrobe/releases/latest/download/$name" "$TMP/$name"
  download 'https://github.com/ThinkerQAQ/wardrobe/releases/latest/download/SHA256SUMS.txt' "$TMP/sha.txt"
  checksha "$TMP/$name" "$TMP/sha.txt"
  mkdir -p "$TMP/payload";tar -xzf "$TMP/$name" -C "$TMP/payload"
  BINARY="$TMP/payload/wardrobe";SRC="$TMP/payload"
 fi
fi
[[ -x "$BINARY" ]] || fail 'Wardrobe binary missing; provide --binary'
mkdir -p "$DEST"/{Data,Private,bin,scripts,Private/profiles}
if [[ "$BINARY" != "$DEST/wardrobe" ]];then
 cp "$BINARY" "$DEST/wardrobe.new";chmod 700 "$DEST/wardrobe.new";mv -f "$DEST/wardrobe.new" "$DEST/wardrobe"
fi
if [[ ! -f "$DEST/Data/config.toml" ]];then
 if [[ -f "$SRC/config.example.toml" ]];then cp "$SRC/config.example.toml" "$DEST/Data/config.toml"
 else printf '[server]\naddress = "127.0.0.1:17936"\nopen_browser = false\n\n[storage]\ndata_dir = "."\n' > "$DEST/Data/config.toml";fi
fi
for name in start-local.sh start-tunnel.sh;do
 [[ -f "$SRC/scripts/$name" ]] || fail "Missing scripts/$name"
 [[ "$SRC/scripts/$name" == "$DEST/scripts/$name" ]] || cp "$SRC/scripts/$name" "$DEST/scripts/$name"
 chmod 700 "$DEST/scripts/$name"
done
if [[ -f "$SRC/brand/wardrobe-mark.svg" ]];then
  mkdir -p "$DEST/brand"
  [[ "$SRC/brand/wardrobe-mark.svg" == "$DEST/brand/wardrobe-mark.svg" ]] || cp "$SRC/brand/wardrobe-mark.svg" "$DEST/brand/wardrobe-mark.svg"
  if [[ -f "$SRC/brand/wardrobe.ico" && "$SRC/brand/wardrobe.ico" != "$DEST/brand/wardrobe.ico" ]];then
    cp "$SRC/brand/wardrobe.ico" "$DEST/brand/wardrobe.ico"
  fi
fi
if [[ -f "$SRC/browser-extension/app/index.html" ]];then
  if [[ "$SRC/browser-extension" != "$DEST/browser-extension" ]];then
    mkdir -p "$DEST/browser-extension"
    cp -R "$SRC/browser-extension/." "$DEST/browser-extension/"
  fi
  rm -f "$DEST/browser-extension/popup.html" "$DEST/browser-extension/popup.js"
  say 'Wardrobe side panel files installed. Load the unpacked extension folder in Edge/Chrome once.'
else say 'Optional browser extension files are not present in this custom source directory.';fi
say 'Installed locally. Existing SQLite and photos preserved.'
autostart(){
 local name="$1" script="$2" unit
 ((NO_START||NO_AUTOSTART))&&return
 if [[ "$OS" == linux ]] && command -v systemctl >/dev/null && systemctl --user show-environment >/dev/null 2>&1;then
  unit="$HOME/.config/systemd/user/wardrobe-$name.service"
  mkdir -p "$(dirname "$unit")"
  printf '[Unit]\nDescription=Wardrobe %s\nAfter=network-online.target\n\n[Service]\nType=simple\nExecStart="%s"\nRestart=on-failure\nRestartSec=8\n\n[Install]\nWantedBy=default.target\n' "$name" "$script" > "$unit"
  systemctl --user daemon-reload
  systemctl --user enable "wardrobe-$name.service" >/dev/null
  systemctl --user restart "wardrobe-$name.service"
 elif [[ "$OS" == darwin ]];then
  unit="$HOME/Library/LaunchAgents/com.thinkerqaq.wardrobe.$name.plist"
  mkdir -p "$(dirname "$unit")"
  local safe
  safe="$(printf '%s' "$script" | sed -e 's/&/\&amp;/g' -e 's/</\&lt;/g' -e 's/>/\&gt;/g' -e 's/"/\&quot;/g')"
  cat > "$unit" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>Label</key><string>com.thinkerqaq.wardrobe.$name</string>
<key>ProgramArguments</key><array><string>$safe</string></array>
<key>RunAtLoad</key><true/><key>KeepAlive</key><true/></dict></plist>
EOF
  launchctl bootout "gui/$(id -u)" "$unit" >/dev/null 2>&1 || true
  launchctl bootstrap "gui/$(id -u)" "$unit" || say "Manual launch required: $script"
 else say "No user-level service manager. Start manually: $script"
 fi
}
if ((NO_START==0));then
 autostart local "$DEST/scripts/start-local.sh"
 if curl -fsS --max-time 3 http://127.0.0.1:17936/api/health | grep -q '"app":"wardrobe"';then say 'Local Web UI: http://127.0.0.1:17936'
 else say 'Service not ready. Check the local process/service and port 17936.';fi
fi
CONNECT=0
if ((NO_TUNNEL==0&&NONINTERACTIVE==0));then
 read -r -p 'Connect to ChatGPT using your own OpenAI Tunnel? [Y/n] ' ans
 [[ -z "$ans" || "$ans" =~ ^[Yy]$ ]] && CONNECT=1
fi
if ((CONNECT));then
 client="$DEST/bin/tunnel-client"
 if [[ ! -x "$client" ]];then
  command -v unzip >/dev/null || fail 'unzip required'
  say 'Downloading official tunnel-client and verifying SHA256'
  tag="$(curl -LsS -o /dev/null -w '%{url_effective}' https://github.com/openai/tunnel-client/releases/latest)"
  tag="$(basename "$tag")"
  [[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail 'Unable to determine official tunnel-client release version'
  name="tunnel-client-$tag-$OS-$ARCH.zip"
  url="https://github.com/openai/tunnel-client/releases/download/$tag"
  download "$url/$name" "$TMP/$name"
  download "$url/SHA256SUMS.txt" "$TMP/tunnel-sha.txt"
  checksha "$TMP/$name" "$TMP/tunnel-sha.txt"
  mkdir -p "$TMP/tunnel";unzip -q "$TMP/$name" -d "$TMP/tunnel"
  found="$(find "$TMP/tunnel" -type f -name tunnel-client | head -1)"
  [[ -n "$found" ]] || fail 'Official tunnel-client binary missing'
  cp "$found" "$client";chmod 700 "$client"
 fi
 idfile="$DEST/Private/tunnel-id"
 if [[ -f "$idfile" ]];then id="$(cat "$idfile")"
 else
  say 'Create a personal Tunnel ID in Platform Tunnel Settings (account permission required).'
  openurl 'https://platform.openai.com/settings/organization/tunnels'
  read -r -p 'Paste your own tunnel_id: ' id
 fi
 [[ "$id" =~ ^tunnel_[a-z0-9]{32}$ ]] || fail 'Invalid tunnel_id'
 printf '%s' "$id" > "$idfile";chmod 600 "$idfile"
 secret="$DEST/Private/tunnel-key"
 if [[ ! -f "$secret" ]];then
  openurl 'https://platform.openai.com/settings/organization/api-keys'
  read -r -s -p 'Paste restricted runtime API key (hidden): ' key;printf '\n'
  (( $(printf '%s' "$key" | wc -c) >= 12 )) || fail 'Runtime API key missing'
  printf '%s' "$key" > "$secret"
  chmod 600 "$secret";unset key
  say 'Runtime key saved only in a local user-owned 0600 file under Private/.'
 fi
 CONTROL_PLANE_API_KEY="$(cat "$secret")"
 export CONTROL_PLANE_API_KEY
 cmd="\"$DEST/wardrobe\" -mcp-stdio -config \"$DEST/Data/config.toml\""
 "$client" init --force --sample sample_mcp_stdio_local --profile wardrobe --profile-dir "$DEST/Private/profiles" --tunnel-id "$id" --mcp-command "$cmd" --health-listen-addr 127.0.0.1:0 --control-plane-api-key-ref env:CONTROL_PLANE_API_KEY
 "$client" doctor --profile wardrobe --profile-dir "$DEST/Private/profiles" --explain || fail 'Tunnel doctor failed. Check permissions and rerun'
 unset CONTROL_PLANE_API_KEY
 autostart tunnel "$DEST/scripts/start-tunnel.sh"
 say 'Finish in ChatGPT: Plugins > + > Add custom MCP server > Tunnel. Select this personal tunnel and approve.'
 openurl 'https://chatgpt.com/'
else say 'Tunnel skipped. Rerun setup later to connect.';fi
if ((NO_START==0&&NONINTERACTIVE==0));then openurl 'http://127.0.0.1:17936';fi
say 'Setup finished. Verify ChatGPT tool discovery with a read-only wardrobe search.'
