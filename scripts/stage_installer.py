#!/usr/bin/env python3
"""Stage Wardrobe Core without any local runtime data or private adapters."""
import argparse
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess
import sys

ROOT=Path(__file__).resolve().parent.parent
FILES=("VERSION","config.example.toml","README.md","README_ZH.md","LICENSE","THIRD_PARTY_NOTICES.md")
DIRS=("browser-extension","brand")
def stage(dest,goos,goarch):
    if dest.exists() and any(dest.iterdir()): raise RuntimeError("stage must be empty")
    dest.mkdir(parents=True,exist_ok=True)
    source_version=(ROOT/"VERSION").read_text().strip()
    manifest=json.loads((ROOT/"browser-extension/manifest.json").read_text())
    if manifest["version"]!=source_version:raise RuntimeError("extension and program version differ")
    binary="wardrobe.exe" if goos=="windows" else "wardrobe"
    env=dict(os.environ,GOOS=goos,GOARCH=goarch,CGO_ENABLED="0")
    version_flags=f"-s -w -X main.appVersion={source_version}"
    subprocess.run(["go","build","-trimpath",f"-ldflags={version_flags}","-o",str(dest/binary),"."],cwd=ROOT,env=env,check=True)
    if goos=="windows":
        subprocess.run(["go","build","-trimpath",f"-ldflags={version_flags} -H=windowsgui","-o",str(dest/"wardrobe-host.exe"),"."],cwd=ROOT,env=env,check=True)

    for f in FILES:shutil.copy2(ROOT/f,dest/f)
    for name in DIRS:shutil.copytree(ROOT/name,dest/name)
    if goos=="darwin":
        subprocess.run(["go","build","-trimpath","-ldflags=-s -w","-o",str(dest/"wardrobe-open"),"./cmd/wardrobe-open"],cwd=ROOT,env=env,check=True)
    for path in (dest/"wardrobe",):
        if path.is_file():path.chmod(0o755)
    # Explicitly forbid data, secrets, user photos or retailer adapters.
    for path in dest.rglob("*"):
        if path.is_symlink():raise RuntimeError("symlink in stage")
        if path.is_file() and (path.name.lower() in ("config.toml","runtime.env","runtime-key.dpapi") or path.suffix.lower() in (".db",".sqlite",".log",".pem",".key")):
            raise RuntimeError("unexpected private file: "+str(path))
    return dest
def main():
    parser=argparse.ArgumentParser()
    parser.add_argument("--os",required=True,choices=("windows","linux","darwin"))
    parser.add_argument("--arch",default="amd64")
    parser.add_argument("--output",type=Path,required=True)
    args=parser.parse_args()
    stage(args.output.resolve(),args.os,args.arch)
    print("CLEAN_WARDROBE_STAGE_OK",args.os,args.arch,(ROOT/"VERSION").read_text().strip())
if __name__=="__main__":main()
