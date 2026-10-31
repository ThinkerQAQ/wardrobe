#!/usr/bin/env bash
# Developer-only regeneration of deterministic app icons from source SVG.
# Published assets are committed; consumers do not need ffmpeg or Python.
set -euo pipefail
cd "$(dirname "$0")/.."
command -v ffmpeg >/dev/null || { echo 'ffmpeg is required to regenerate icons' >&2; exit 1; }
mkdir -p browser-extension/icons web brand
cp brand/wardrobe-mark.svg browser-extension/icons/icon.svg
cp brand/wardrobe-mark.svg web/favicon.svg
for dim in 16 32 48 128 256; do
  ffmpeg -hide_banner -loglevel error -y -f image2 -i brand/wardrobe-mark.svg -vf "scale=${dim}:${dim}:flags=lanczos" "browser-extension/icons/icon${dim}.png"
done
python3 - <<'PY'
# Windows ICO supports PNG-encoded image payloads. Embed the same brand mark.
import pathlib,struct
icons=[pathlib.Path(f'browser-extension/icons/icon{size}.png').read_bytes() for size in (16,32,48,128,256)]
sizes=(16,32,48,128,256)
entry_len=16*len(icons)
header=struct.pack('<HHH',0,1,len(icons))
offset=6+entry_len
entries=[]
for size,blob in zip(sizes,icons):
 entries.append(struct.pack('<BBBBHHII',size if size<256 else 0,size if size<256 else 0,0,0,1,32,len(blob),offset))
 offset+=len(blob)
pathlib.Path('brand/wardrobe.ico').write_bytes(header+b''.join(entries)+b''.join(icons))
PY
printf 'Generated Wardrobe favicon, Windows icon, and Edge/Chrome extension icons.\n'
