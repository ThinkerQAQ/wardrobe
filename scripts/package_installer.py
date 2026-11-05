#!/usr/bin/env python3
"""Build native Qt Installer Framework installers from a clean Wardrobe release stage."""
import argparse
import datetime
import pathlib
import platform
import re
import shutil
import subprocess
import tempfile
import xml.etree.ElementTree as ET

ROOT=pathlib.Path(__file__).resolve().parent.parent
ASSETS=ROOT/"scripts"/"installer"
PAYLOAD=("wardrobe.exe","wardrobe-host.exe","wardrobe","browser-extension","brand","config.example.toml",
         "README.md","README_ZH.md","LICENSE","THIRD_PARTY_NOTICES.md","THIRD_PARTY_LICENSES",
         "scripts","Setup-Wardrobe.cmd")
EXCLUDED={"data","private","backups",".git","runtime.env","runtime-key.dpapi","config.toml"}
def xml(path,tag,entries):
    node=ET.Element(tag)
    for k,v in entries.items(): ET.SubElement(node,k).text=v
    ET.indent(node)
    ET.ElementTree(node).write(path,encoding="utf-8",xml_declaration=True)
def prepare(stage, out, version, target):
    if not re.fullmatch(r"\d+\.\d+\.\d+",version): raise ValueError("invalid version")
    import json
    manifest=stage/"browser-extension/manifest.json"
    if manifest.is_file() and json.loads(manifest.read_text())["version"]!=version:
        raise ValueError("extension and installer version differ")
    exe="wardrobe.exe" if target=="windows" else "wardrobe"
    for name in (exe,"config.example.toml","LICENSE","browser-extension/manifest.json"):
        if not (stage/name).is_file():raise ValueError("stage missing "+name)
    for path in stage.rglob("*"):
        if path.is_symlink():raise ValueError("stage contains a symlink")
        if any(part.lower() in EXCLUDED for part in path.relative_to(stage).parts):
            raise ValueError("private runtime content staged: "+str(path))
        if path.is_file() and path.suffix.lower() in (".db",".sqlite",".key",".pem",".log",".har",".sqlite3"):
            raise ValueError("private data staged: "+str(path))
    conf=out/"config"
    meta=out/"packages/com.thinkerqaq.wardrobe/meta"
    data=out/"packages/com.thinkerqaq.wardrobe/data"
    conf.mkdir(parents=True);meta.mkdir(parents=True);data.mkdir(parents=True)
    for name in PAYLOAD:
        src=stage/name
        if src.is_dir():shutil.copytree(src,data/name)
        elif src.is_file():shutil.copy2(src,data/name)
    for name in ("controller.qs","style.qss"):
        shutil.copy2(ASSETS/name,conf/name)
    for name in ("first-run.ui",):
        shutil.copy2(ASSETS/name,meta/name)
    shutil.copy2(stage/"LICENSE",meta/"license.txt")
    logo=stage/"browser-extension/icons/icon128.png"
    if not logo.is_file():raise ValueError("stage missing browser icon")
    shutil.copy2(logo,conf/"wardrobe-logo.png")
    shutil.copy2(logo,conf/"wardrobe-window.png")
    if target=="windows":
        ico=stage/"brand/wardrobe.ico"
        if ico.is_file():shutil.copy2(ico,conf/"wardrobe-application.ico")
    values={"Name":"Wardrobe","Version":version,"Title":"Wardrobe","Publisher":"ThinkerQAQ",
            "StartMenuDir":"Wardrobe","TargetDir":"@HomeDir@/Wardrobe",
            "MaintenanceToolName":"Wardrobe-Maintenance","RemoveTargetDir":"false",
            "WizardStyle":"Modern","WizardShowPageList":"false",
            "WizardDefaultWidth":"680","WizardDefaultHeight":"520",
            "TitleColor":"#345548","ControlScript":"controller.qs","StyleSheet":"style.qss",
            "Logo":"wardrobe-logo.png","InstallerWindowIcon":"wardrobe-window",
            "ProductUrl":"https://github.com/ThinkerQAQ/wardrobe"}
    if target=="windows" and (conf/"wardrobe-application.ico").is_file():
        values["InstallerApplicationIcon"]="wardrobe-application"
    xml(conf/"config.xml","Installer",values)
    xml(meta/"package.xml","Package",{
        "DisplayName":"Wardrobe","Description":"Local wardrobe and outfit workspace",
        "Version":version,"ReleaseDate":datetime.date.today().isoformat(),
        "Default":"true","Script":"installscript.qs"})
    tree=ET.parse(meta/"package.xml")
    ET.SubElement(ET.SubElement(tree.getroot(),"Licenses"),"License",
                  {"name":"MIT License", "file":"license.txt"})
    ET.SubElement(ET.SubElement(tree.getroot(),"UserInterfaces"),"UserInterface").text="first-run.ui"
    ET.indent(tree);tree.write(meta/"package.xml",encoding="utf-8",xml_declaration=True)
    # Native installer and unattended QtIFW install share the same payload.
    script='''function Component() {
    if (installer.isInstaller() && !installer.isCommandLineInstance()) {
        installer.installationFinished.connect(this, Component.prototype.showFirstRunOptions);
        installer.finishButtonClicked.connect(this, Component.prototype.finishFirstRun);
    }
}
Component.prototype.createOperations = function() {
    component.createOperations();
'''
    if target=="windows":
        script+='    component.addOperation("Mkdir", "@UserStartMenuProgramsPath@/Wardrobe");\n'
        # Rebind legacy Native Messaging manifests after files are extracted.
        # The GUI-subsystem host exits immediately and never opens a CMD window.
        script+='    component.addOperation("Execute", "@TargetDir@/wardrobe-host.exe", "--repair-native-host");\n'
        script+='    component.addOperation("CreateShortcut", "@TargetDir@/wardrobe-host.exe", "@UserStartMenuProgramsPath@/Wardrobe/Wardrobe.lnk", "--open-ui", "workingDirectory=@TargetDir@");\n'
    elif target=="linux":
        script+='    component.addOperation("CreateDesktopEntry", "wardrobe.desktop", "Type=Application\\nName=Wardrobe\\nExec=/bin/bash \\"@TargetDir@/scripts/Open-Wardrobe.sh\\"\\nTerminal=false\\nCategories=Utility;");\n'
    else:
        # macOS .dmg contains a launcher app and a maintained installation tree.
        import plistlib
        app=data/"Wardrobe.app/Contents"
        (app/"MacOS").mkdir(parents=True)
        (app/"Info.plist").write_bytes(plistlib.dumps({
            "CFBundleName":"Wardrobe","CFBundleIdentifier":"com.thinkerqaq.wardrobe",
            "CFBundleExecutable":"Wardrobe","CFBundleVersion":version,
            "CFBundlePackageType":"APPL"}))
        launcher=app/"MacOS/Wardrobe"
        launcher.write_text('#!/bin/bash\nset -euo pipefail\nroot="$(cd "$(dirname "$0")/../../.." && pwd)"\nexec /bin/bash "$root/scripts/Open-Wardrobe.sh"\n')
        launcher.chmod(0o755)
    script+='};\n'
    script+=(ASSETS/"finish.qs").read_text().replace("__PLATFORM__",target)
    (meta/"installscript.qs").write_text(script)
    # Data/ and Private/ are deliberately absent from every tracked install operation.

def ensure_windows_gui_subsystem(executable):
    """QtIFW binarycreator emits a console PE; prevent a black CMD window on launch."""
    with executable.open("r+b") as binary:
        if binary.read(2) != b"MZ":
            raise ValueError("Windows installer is not a PE executable")
        binary.seek(0x3c)
        pe_offset=int.from_bytes(binary.read(4),"little")
        if pe_offset < 64 or pe_offset > executable.stat().st_size - 96:
            raise ValueError("Windows installer has an invalid PE offset")
        binary.seek(pe_offset)
        if binary.read(4) != b"PE\0\0":
            raise ValueError("Windows installer has no PE signature")
        optional_header=pe_offset + 24
        binary.seek(optional_header)
        if int.from_bytes(binary.read(2),"little") not in (0x10b,0x20b):
            raise ValueError("Windows installer has an unsupported PE header")
        subsystem_offset=optional_header + 68
        binary.seek(subsystem_offset)
        subsystem=int.from_bytes(binary.read(2),"little")
        if subsystem not in (2,3):
            raise ValueError("Windows installer is neither GUI nor console")
        if subsystem == 3:
            binary.seek(subsystem_offset)
            binary.write((2).to_bytes(2,"little"))
        # Only the PE subsystem is changed; the appended Qt installer payload is intact.

def main():
    p=argparse.ArgumentParser()
    p.add_argument("--stage",type=pathlib.Path,required=True)
    p.add_argument("--version",required=True)
    p.add_argument("--os",choices=("windows","linux","darwin"),required=True)
    p.add_argument("--output",type=pathlib.Path,required=True)
    p.add_argument("--binarycreator",default="binarycreator")
    p.add_argument("--prepare-only",action="store_true")
    a=p.parse_args()
    if a.output.exists():p.error("refusing to overwrite existing artifact")
    if a.prepare_only:
        prepare(a.stage.resolve(),a.output.resolve(),a.version,a.os)
        return
    host={"Windows":"windows","Linux":"linux","Darwin":"darwin"}[platform.system()]
    if host!=a.os:p.error("native Qt installers must be built on the matching OS")
    a.output.parent.mkdir(parents=True,exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="wardrobe-qtifw-") as tmp:
        path=pathlib.Path(tmp)
        prepare(a.stage.resolve(),path,a.version,a.os)
        subprocess.run([a.binarycreator,"--offline-only","-c",str(path/"config/config.xml"),
                        "-p",str(path/"packages"),str(a.output.resolve())],check=True)
    if a.os == "windows":
        ensure_windows_gui_subsystem(a.output.resolve())
if __name__=="__main__":main()
