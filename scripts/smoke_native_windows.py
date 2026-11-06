#!/usr/bin/env python3
"""Windows native install smoke; no PowerShell dependency."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import time
from urllib.request import urlopen

p = argparse.ArgumentParser()
p.add_argument("--install-dir", type=Path, required=True)
args = p.parse_args()
root = args.install_dir.resolve()
exe = root / "wardrobe.exe"
if not exe.is_file():
    raise SystemExit("Wardrobe executable not installed")
expected_version = (root / "VERSION").read_text(encoding="utf-8").strip()
reported_version = subprocess.check_output([str(exe), "version"], text=True, timeout=10).strip()
if reported_version != expected_version:
    raise RuntimeError(f"Wardrobe binary version mismatch: {reported_version} != {expected_version}")
proc = subprocess.Popen([str(exe), "-open=false"], cwd=root,
    stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
    creationflags=getattr(subprocess, "CREATE_NO_WINDOW", 0))
try:
    for _ in range(60):
        if proc.poll() is not None:
            raise RuntimeError(f"Wardrobe exited: {proc.returncode}")
        try:
            with urlopen("http://127.0.0.1:17936/api/health", timeout=2) as response:
                health = json.load(response)
            if health.get("app") == "wardrobe" and health.get("status") == "ok" and health.get("version") == expected_version:
                break
        except Exception:
            pass
        time.sleep(0.25)
    else:
        raise RuntimeError("Wardrobe first run did not become healthy")
    if not (root/"Data/config.toml").is_file():
        raise RuntimeError("Wardrobe did not create Data/config.toml")
    print("WINDOWS_FIRST_RUN_HEALTH_OK")
finally:
    proc.terminate()
    try: proc.wait(timeout=8)
    except subprocess.TimeoutExpired: proc.kill(); proc.wait()
