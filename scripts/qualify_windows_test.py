"""Public Windows runner checks, on disposable native CI with synthetic account state."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

SCRIPTS = Path(__file__).resolve().parent
VERSION = "v0.1.0-rc.1"


@unittest.skipUnless(os.name == "nt", "native Windows required")
class WindowsQualificationTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        if os.environ.get("GITHUB_ACTIONS") != "true":
            raise unittest.SkipTest("disposable CI only: tests change persistent user PATH")
        cls.compiled = tempfile.TemporaryDirectory(prefix="tofa native fixtures ")
        root = Path(cls.compiled.name)
        # A real .exe boundary exercises Go/Python Windows executable discovery.
        source = r'''
using System;
using System.Diagnostics;
using System.IO;
using System.Text;
class Fixture {
 static string Q(string value) {
  var b = new StringBuilder("\""); int slashes = 0;
  foreach(char c in value) {
   if(c == '\\') { slashes++; continue; }
   b.Append('\\', c == '"' ? slashes * 2 + 1 : slashes); b.Append(c); slashes = 0;
  }
  b.Append('\\', slashes * 2); return b.Append('"').ToString();
 }
 static int Main(string[] args) {
  var arguments = Q(SCRIPT) + " " + Q(Path.GetFileName(Process.GetCurrentProcess().MainModule.FileName));
  foreach(var arg in args) arguments += " " + Q(arg);
  var start = new ProcessStartInfo(PYTHON, arguments); start.UseShellExecute = false;
  var child = Process.Start(start); child.WaitForExit(); return child.ExitCode;
 }
}
'''.replace("SCRIPT", '@"' + str(SCRIPTS / "fixtures/windows_qualification.py").replace('"', '""') + '"').replace("PYTHON", '@"' + sys.executable.replace('"', '""') + '"')
        (root / "fixture.cs").write_text(source)
        compiler = Path(os.environ["SystemRoot"]) / "Microsoft.NET/Framework64/v4.0.30319/csc.exe"
        cls.exe = root / "fixture.exe"
        subprocess.run([str(compiler), "/nologo", "/out:" + str(cls.exe), str(root / "fixture.cs")], check=True, capture_output=True)

    @classmethod
    def tearDownClass(cls):
        cls.compiled.cleanup()

    def setUp(self):
        import winreg
        self.temp = tempfile.TemporaryDirectory(prefix="tofa synthetic Windows ")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.assets = self.root / "assets"
        self.bin = self.root / "fixtures"
        self.local = self.root / "local"
        self.config = self.local / "tofa"
        self.install = self.config / "install"
        for folder in (self.assets, self.bin, self.config, self.root / ".codex", self.root / "helpers"):
            folder.mkdir(parents=True)
        with winreg.OpenKey(winreg.HKEY_CURRENT_USER, "Environment", 0, winreg.KEY_ALL_ACCESS) as key:
            try:
                self.original_path = winreg.QueryValueEx(key, "Path")
            except FileNotFoundError:
                self.original_path = None
        def restore():
            with winreg.OpenKey(winreg.HKEY_CURRENT_USER, "Environment", 0, winreg.KEY_ALL_ACCESS) as key:
                if self.original_path is None:
                    try: winreg.DeleteValue(key, "Path")
                    except FileNotFoundError: pass
                else: winreg.SetValueEx(key, "Path", 0, self.original_path[1], self.original_path[0])
        self.addCleanup(restore)
        self.report = self.root / "report.json"
        self.env = {**os.environ, "USERPROFILE": str(self.root), "HOME": str(self.root),
                    "LOCALAPPDATA": str(self.local), "APPDATA": str(self.root / "roaming"),
                    "CODEX_HOME": str(self.root / ".codex"), "TEMP": str(self.root / "helpers"),
                    "TMP": str(self.root / "helpers"), "PATH": str(self.bin) + os.pathsep + os.environ["PATH"],
                    "FIXTURE_ROOT": str(self.root), "FIXTURE_MODE": "success", "TOFA_INSTALL_DIR": ""}
        for name in ("gh.exe", "codex.exe"):
            shutil.copyfile(self.exe, self.bin / name)
        self.asset = "tofa_" + VERSION + "_windows_amd64.exe"
        shutil.copyfile(self.exe, self.assets / self.asset)
        for name in ("install.ps1", "uninstall.ps1"):
            shutil.copyfile(SCRIPTS / name, self.assets / name)
        (self.config / "unrelated.txt").write_text("PRIVATE_NEIGHBOR")
        (self.root / ".codex/auth.json").write_text("PRIVATE_AUTH")
        self.checksums()

    def checksums(self):
        (self.assets / "SHA256SUMS").write_text("".join(
            hashlib.sha256(p.read_bytes()).hexdigest() + "  " + p.name + "\n"
            for p in self.assets.iterdir() if p.name != "SHA256SUMS"))

    def run_runner(self, answers="READY\nFOUND\nPURGE\n", timeout=20):
        result = subprocess.run([sys.executable, str(SCRIPTS / "qualify_windows.py"),
                                 "--version", VERSION, "--output", str(self.report),
                                 "--timeout", str(timeout)], input=answers, text=True, capture_output=True,
                                env=self.env, timeout=120)
        self.assertTrue(self.report.is_file(), result.stdout + result.stderr)
        report = json.loads(self.report.read_text())
        for value in ("PRIVATE_AUTH", "PRIVATE_KEY", "PRIVATE_NEIGHBOR", "PRIVATE_CONVERSATION",
                      str(self.root), hashlib.sha256(b"PRIVATE_KEY").hexdigest(),
                      "11111111-1111-4111-8111-111111111111"):
            self.assertNotIn(value, self.report.read_text())
            self.assertNotIn(value, self.report.with_suffix(".md").read_text())
        return result, report

    def test_download_install_live_reinstall_reuse_and_purge(self):
        result, report = self.run_runner()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr + json.dumps(report))
        self.assertEqual(report["outcome"], "passed")
        self.assertEqual(report["candidate"]["commit"], "1234567890abcdef1234567890abcdef12345678")
        self.assertEqual(report["backend"], "file")
        self.assertEqual((self.root / "logins").read_text(), "login\n")
        self.assertTrue(all(report["preservation"].values()))
        self.assertTrue(all(report["cleanup"].values()))
        self.assertTrue(report["live"]["same_session"])
        self.assertTrue(report["saved_login_reuse"]["passed"])
        self.assertTrue(all(stage["status"] == "passed" for stage in report["stages"]))


if __name__ == "__main__":
    unittest.main()
