#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
url="http://127.0.0.1:17936"
healthy() { curl -fsS --connect-timeout 2 --max-time 3 "$url/api/health" 2>/dev/null | grep -q '"app":"wardrobe"'; }
if ! healthy; then
  mkdir -p "$root/Data"
  (cd "$root" && nohup "$root/wardrobe" -open=false >/dev/null 2>&1 < /dev/null &)
  for i in $(seq 1 48); do healthy && break; sleep .25; done
  healthy || { echo "Wardrobe service did not become healthy on port 17936" >&2; exit 1; }
fi
if [[ "$(uname -s)" == Darwin ]]; then open "$url"
else command -v xdg-open >/dev/null && xdg-open "$url" >/dev/null 2>&1 || printf '%s\n' "$url"; fi
