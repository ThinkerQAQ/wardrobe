#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
for file in browser-extension/collector-main.js browser-extension/collector-content.js browser-extension/order-parser.js browser-extension/bridge-local.js order_retailer_match.go order_retailer_reconcile.go order_retailer_init.go order_legacy_reconcile.go orders_cross_source_test.go;do
 if [[ -e "$file" ]];then echo "PRIVATE ADAPTER IN PUBLIC TREE: $file" >&2;exit 1;fi
done
python3 - <<'PYCODE'
from pathlib import Path
import json,re
p=Path('browser-extension/manifest.json')
m=json.loads(p.read_text())
if m.get('host_permissions')!=['http://127.0.0.1:17936/*'] or m.get('content_scripts')!=[]:
    raise SystemExit('Public extension has source-specific site access')
if set(m.get('permissions',[]))-{'storage','sidePanel'}:
    raise SystemExit('Public extension has unexpected permissions')
patterns=[r'mtop\.taobao\.order',r'pcorder_center_list',r'h5api\.m\.taobao',r'wardrobe\.orderCapture',r'queryboughtlist',r'360buyimg\.com']
for path in [Path('browser-extension'),Path('web'),Path('docs'),Path('skills')]:
    for f in path.rglob('*'):
        if not f.is_file() or f.suffix not in ('.js','.json','.md','.html'):continue
        s=f.read_text(encoding='utf-8',errors='replace')
        for pat in patterns:
            if re.search(pat,s,re.I):raise SystemExit(f'Private-source marker in public bundle: {f}')
if Path('Data').is_dir(): # local runtime data is ignored; it must never be in tracked files
    pass
PYCODE
if git rev-parse --is-inside-work-tree >/dev/null 2>&1;then
 if git ls-files | grep -E '^(Data/|Private/)|\.sqlite($|-)|\.db($|-)|(^|/)\.env($|\.)' >/dev/null;then
    echo 'Private runtime path is tracked' >&2;exit 1
 fi
fi
echo "Public-source boundary: PASS"
