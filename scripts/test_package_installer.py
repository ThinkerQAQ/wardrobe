#!/usr/bin/env python3
import importlib.util
from pathlib import Path
import tempfile
import unittest
import xml.etree.ElementTree as ET
spec=importlib.util.spec_from_file_location("p",Path(__file__).with_name("package_installer.py"))
p=importlib.util.module_from_spec(spec);spec.loader.exec_module(p)
class InstallerTests(unittest.TestCase):
 def fixture(self,root,osname):
  src=root/"stage"
  (src/"browser-extension/icons").mkdir(parents=True)
  (src/"browser-extension/manifest.json").write_text('{"version":"1.1.4"}')
  (src/"browser-extension/icons/icon128.png").write_bytes(b"fake-icon")
  (src/"brand").mkdir()
  (src/"brand/wardrobe.ico").write_bytes(b"fake-ico")
  for name in ("config.example.toml","README.md","README_ZH.md","LICENSE","THIRD_PARTY_NOTICES.md"):
   (src/name).write_text("test")
  (src/("wardrobe.exe" if osname=="windows" else "wardrobe")).write_bytes(b"stub")
  if osname=="windows":
   (src/"wardrobe-host.exe").write_bytes(b"native-host-stub")
  if osname=="darwin": (src/"wardrobe-open").write_bytes(b"native-macos-launcher")
  return src
 def test_platform_layout_and_unmanaged_data(self):
  for osname in ("windows","linux","darwin"):
   with self.subTest(osname=osname),tempfile.TemporaryDirectory() as td:
    root=Path(td);stage=self.fixture(root,osname);dst=root/"installer"
    p.prepare(stage,dst,"1.1.4",osname)
    cfg=ET.parse(dst/"config/config.xml").getroot()
    self.assertEqual(cfg.findtext("RemoveTargetDir"),"false")
    package=dst/"packages/com.thinkerqaq.wardrobe/data"
    self.assertFalse((package/"Data").exists())
    self.assertFalse((package/"Private").exists())
    self.assertTrue((package/("wardrobe.exe" if osname=="windows" else "wardrobe")).exists())
    self.assertFalse(list(package.rglob("*.ps1")))
    self.assertFalse(list(package.rglob("*.cmd")))
    self.assertFalse((package/"Setup-Wardrobe.cmd").exists())
    script=(dst/"packages/com.thinkerqaq.wardrobe/meta/installscript.qs").read_text()
    self.assertNotIn("/bin/bash",script)
    self.assertFalse(list(package.rglob("*.sh")))
    if osname=="windows":
     self.assertIn('--repair-native-host',script)
     self.assertIn('wardrobe-host.exe',script)
     self.assertIn('--open-ui',script)
     self.assertIn('--guide-extension',script)
     self.assertNotIn('WindowsPowerShell',script)
    if osname=="darwin":self.assertEqual((package/"Wardrobe.app/Contents/MacOS/Wardrobe").read_bytes(),b"native-macos-launcher")
    if osname=="linux":self.assertIn("CreateDesktopEntry",script)
 def test_rejects_data_and_secrets(self):
  for file in ("Data/something.db","Private/key","config.toml","scripts/runtime.env","browser-extension/admin.key"):
   with self.subTest(file=file),tempfile.TemporaryDirectory() as td:
    root=Path(td);src=self.fixture(root,"windows");f=src/file
    f.parent.mkdir(parents=True,exist_ok=True);f.write_text("secret")
    with self.assertRaises(ValueError):p.prepare(src,root/"installer","1.1.4","windows")
 def test_windows_console_binary_becomes_gui_without_touching_payload(self):
  with tempfile.TemporaryDirectory() as td:
   exe=Path(td)/"Wardrobe-Setup.exe"
   content=bytearray(512)
   content[:2]=b"MZ"
   content[0x3c:0x40]=(0x80).to_bytes(4,"little")
   content[0x80:0x84]=b"PE\0\0"
   content[0x98:0x9a]=(0x20b).to_bytes(2,"little")
   offset=0x98+68
   content[offset:offset+2]=(3).to_bytes(2,"little")
   original=bytes(content)+b"QTIFW_PAYLOAD_STAYS_UNCHANGED"
   exe.write_bytes(original)
   p.ensure_windows_gui_subsystem(exe)
   modified=exe.read_bytes()
   self.assertEqual(modified[offset:offset+2],(2).to_bytes(2,"little"))
   self.assertEqual(modified[:offset]+modified[offset+2:],original[:offset]+original[offset+2:])
   p.ensure_windows_gui_subsystem(exe)
   self.assertEqual(exe.read_bytes(),modified)
 def test_windows_gui_patch_rejects_invalid_executable(self):
  with tempfile.TemporaryDirectory() as td:
   exe=Path(td)/"invalid.exe"
   exe.write_bytes(b"invalid executable")
   with self.assertRaises(ValueError):p.ensure_windows_gui_subsystem(exe)
if __name__=="__main__": unittest.main()
