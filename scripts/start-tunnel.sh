#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SECRET="$ROOT/Private/tunnel-key"
CLIENT="$ROOT/bin/tunnel-client"
PROFILE="$ROOT/Private/profiles"
[[ -f "$SECRET" && -x "$CLIENT" ]] || { echo 'Wardrobe tunnel not configured; run setup.sh' >&2;exit 2; }
CONTROL_PLANE_API_KEY="$(cat "$SECRET")"
export CONTROL_PLANE_API_KEY
exec "$CLIENT" run --profile wardrobe --profile-dir "$PROFILE"
