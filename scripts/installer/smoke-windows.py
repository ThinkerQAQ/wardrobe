"""Exercise the native Qt wizard on a private Windows desktop, then uninstall.
Only synthetic state is written. No browser or application is launched at Finish.
"""
import argparse
import ctypes as c
from ctypes import wintypes as w
import os
from pathlib import Path
import subprocess
import time

class Startup(c.Structure):
    _fields_=[("cb",w.DWORD),("reserved",w.LPWSTR),("desktop",w.LPWSTR),("title",w.LPWSTR),
              ("x",w.DWORD),("y",w.DWORD),("width",w.DWORD),("height",w.DWORD),
              ("charsx",w.DWORD),("charsy",w.DWORD),("fill",w.DWORD),("flags",w.DWORD),
              ("show",w.WORD),("reserved2",w.WORD),("reservedptr",c.POINTER(c.c_byte)),
              ("stdin",w.HANDLE),("stdout",w.HANDLE),("stderr",w.HANDLE)]
class Process(c.Structure):
    _fields_=[("process",w.HANDLE),("thread",w.HANDLE),("pid",w.DWORD),("tid",w.DWORD)]

def main():
    if os.name!="nt": raise SystemExit("Run with native Windows Python")
    p=argparse.ArgumentParser()
    p.add_argument("--installer",type=Path,required=True)
    p.add_argument("--workdir",type=Path,required=True)
    a=p.parse_args()
    with a.installer.open("rb") as binary:
        if binary.read(2) != b"MZ":
            raise AssertionError("Windows installer is not a PE executable")
        binary.seek(0x3c)
        pe_offset=int.from_bytes(binary.read(4),"little")
        binary.seek(pe_offset+24+68)
        subsystem=int.from_bytes(binary.read(2),"little")
        if subsystem != 2:
            raise AssertionError(f"Installer must be GUI subsystem (2), got {subsystem}")
    print("WINDOWS_GUI_SUBSYSTEM_OK",flush=True)
    qa=a.workdir.resolve()
    qa.mkdir(parents=True,exist_ok=False)
    target=qa/"install with spaces"
    (target/"Data").mkdir(parents=True)
    state={target/"Data/config.toml":b"# retained synthetic configuration\r\n",
           target/"Data/keep.sqlite":b"SYNTHETIC-SENTINEL",
           target/"Private/key.test":b"PRIVATE-SENTINEL"}
    for path,data in state.items():
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)
    source=Path(__file__).parent
    script=qa/"wizard-smoke.qs"
    script.write_text((source/"controller.qs").read_text()+"\n"+(source/"gui-smoke.qs").read_text(),encoding="utf-8")
    u=c.WinDLL("user32",use_last_error=True)
    k=c.WinDLL("kernel32",use_last_error=True)
    u.CreateDesktopW.argtypes=[w.LPCWSTR,w.LPCWSTR,c.c_void_p,w.DWORD,w.DWORD,c.c_void_p]
    u.CreateDesktopW.restype=w.HANDLE
    u.CloseDesktop.argtypes=[w.HANDLE]
    k.CreateProcessW.argtypes=[w.LPCWSTR,w.LPWSTR,c.c_void_p,c.c_void_p,w.BOOL,w.DWORD,c.c_void_p,w.LPCWSTR,c.POINTER(Startup),c.POINTER(Process)]
    k.CreateProcessW.restype=w.BOOL
    k.WaitForSingleObject.argtypes=[w.HANDLE,w.DWORD]
    k.GetExitCodeProcess.argtypes=[w.HANDLE,c.POINTER(w.DWORD)]
    k.TerminateProcess.argtypes=[w.HANDLE,w.UINT]
    k.CloseHandle.argtypes=[w.HANDLE]
    desktop_name="WardrobeWizardQA"+str(os.getpid())
    desktop=u.CreateDesktopW(desktop_name,None,None,0,0x01ff,None)
    if not desktop: raise c.WinError(c.get_last_error())
    os.environ["QT_QPA_PLATFORM"]="windows"
    os.environ.pop("QT_DEBUG_BACKINGSTORE",None)
    def run(args,label):
        startup=Startup()
        startup.cb=c.sizeof(startup)
        startup.desktop=desktop_name
        process=Process()
        command=c.create_unicode_buffer(subprocess.list2cmdline([str(v) for v in args]))
        if not k.CreateProcessW(None,command,None,None,False,0x08000000,None,str(qa),c.byref(startup),c.byref(process)):
            raise c.WinError(c.get_last_error())
        print("QA_PROCESS",label,process.pid,flush=True)
        deadline=time.monotonic()+90
        while k.WaitForSingleObject(process.process,500)==258:
            if time.monotonic()>deadline:
                k.TerminateProcess(process.process,1)
                raise TimeoutError(label)
        code=w.DWORD()
        k.GetExitCodeProcess(process.process,c.byref(code))
        k.CloseHandle(process.thread)
        k.CloseHandle(process.process)
        if code.value: raise RuntimeError(label+" exit="+str(code.value))
    try:
        run([a.installer.resolve(),"--verbose","--root",target,"--script",script,
             "WardrobeQARoot="+str(target),"--auto-answer","OverwriteTargetDirectory=Yes",
             "UserStartMenuProgramsPath="+str(qa/"menu")],"GUI")
        log=(target/"InstallationLog.txt").read_text(encoding="utf-8",errors="replace")
        for marker in ("QA_INTRO","QA_TARGET","QA_LICENSE","QA_READY","QA_FINISHED","QA_FINISH_OPTIONS","WARDROBE_FINISH_OPTIONS"):
            if marker not in log: raise AssertionError("Missing "+marker)
        if "QA_BAD_" in log: raise AssertionError("Wizard regression; see InstallationLog.txt")
        for path,data in state.items():
            assert path.read_bytes()==data,"State overwritten: "+str(path)
        assert (target/"wardrobe.exe").is_file()
        assert (target/"wardrobe-host.exe").is_file()
        assert not list(target.rglob("*.ps1")), "native installer must not carry PowerShell"
        assert not list(target.rglob("*.cmd")), "native installer must not carry legacy CMD launchers"
        print("QA_GUI_PASS",flush=True)
    finally:
        if (target/"Wardrobe-Maintenance.exe").is_file():
            run([target/"Wardrobe-Maintenance.exe","--confirm-command","purge"],"uninstall")
            for path,data in state.items():
                assert path.read_bytes()==data,"State removed: "+str(path)
            print("QA_UNINSTALL_PRESERVED_STATE",flush=True)
        u.CloseDesktop(desktop)

if __name__=="__main__": main()
