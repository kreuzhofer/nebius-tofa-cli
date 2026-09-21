"""Offline native lifecycle checks against an actual versioned distribution.

Usage: python3 scripts/lifecycle_test.py dist v0.1.0-rc.1
Uses macOS zsh / Linux bash with disposable homes and synthetic file credentials.
No native credential store, installed Codex, or inference service is used.
"""
import hashlib
import os
import pathlib
import platform
import shutil
import subprocess
import sys
import tempfile
import unittest


class LifecycleTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="tofa lifecycle ' ")
        self.addCleanup(self.temp.cleanup)
        self.home = pathlib.Path(self.temp.name)
        self.install = self.home / ".local/share/tofa"
        self.binary = self.install / "bin/tofa"
        self.config = self.home / ".config/tofa"
        self.assets = self.home / "release"
        self.assets.mkdir()
        system = platform.system().lower()
        arch = {"arm64": "arm64", "aarch64": "arm64", "x86_64": "amd64"}[platform.machine()]
        self.asset = f"tofa_{VERSION}_{system}_{arch}"
        for name in (self.asset, "SHA256SUMS", "install.sh"):
            shutil.copyfile(DIST / name, self.assets / name)
        self.shell = "/bin/zsh" if system == "darwin" else "/bin/bash"
        self.rc = self.home / (".zshrc" if system == "darwin" else ".bashrc")
        self.startup = [self.rc]
        if system == "linux":
            self.startup.append(self.home / ".profile")
        self.original_rc = "# Unrelated shell configuration\nexport LIFECYCLE_SENTINEL=retained\n"
        for rc in self.startup:
            rc.write_text(self.original_rc)
        # An allowlist prevents inherited tofa, shell, and credential settings
        # from redirecting the test into the user's real state or installation.
        self.env = {
            "HOME": str(self.home), "XDG_CONFIG_HOME": str(self.home / ".config"),
            "XDG_DATA_HOME": str(self.home / ".local/share"),
            "XDG_CACHE_HOME": str(self.home / ".cache"),
            "ZDOTDIR": str(self.home), "SHELL": self.shell,
            "PATH": os.defpath, "TERM": "dumb", "LC_ALL": "C",
            "TMPDIR": str(self.home), "TOFA_RELEASE_BASE_URL": self.assets.as_uri(),
        }
        self.config.mkdir(parents=True, mode=0o700)
        reference = "0123456789abcdef0123456789abcdef"
        self.saved = {
            self.config / "config.yml": (
                "version: 1\nproject_id: synthetic-project\nmodel: synthetic-model\n"
                f"credential_backend: file\ncredential_ref: '{reference}'\n"
            ).encode(),
            self.config / "credentials.yml": f"'{reference}': synthetic-key\n".encode(),
        }
        for path, content in self.saved.items():
            path.write_bytes(content)
            path.chmod(0o600)
        self.unrelated = {}
        for name in (".config/tofa/unrelated.txt", ".codex/config.toml", ".codex/auth.json"):
            path = self.home / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(b"unrelated synthetic state\n")
            self.unrelated[path] = path.read_bytes()

    def command(self, args, success=True):
        result = subprocess.run(args, env=self.env, cwd=self.home, text=True,
                                capture_output=True, timeout=30)
        if success:
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        else:
            self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        return result

    def install_candidate(self, success=True):
        return self.command(["sh", str(self.assets / "install.sh"), "--version", VERSION], success)

    def fresh_shell(self, command, success=True):
        # Interactive startup reads .zshrc/.bashrc itself: no manual sourcing or
        # injected install directory on PATH can make discovery pass accidentally.
        args = [self.shell]
        if self.shell.endswith("bash"):
            args.append("--noprofile")
        return self.command([*args, "-ic", command], success)

    def assert_saved_state(self):
        for path, content in self.saved.items():
            self.assertEqual(path.read_bytes(), content)

    def assert_unrelated_state(self):
        for path, content in self.unrelated.items():
            self.assertEqual(path.read_bytes(), content)
        for rc in self.startup:
            self.assertIn(self.original_rc, rc.read_text())
        self.assertEqual(self.fresh_shell('printf "%s" "$LIFECYCLE_SENTINEL"').stdout, "retained")

    def assert_available(self):
        self.assertEqual(self.fresh_shell("command -v tofa").stdout.strip(), str(self.binary))
        self.assertEqual(self.fresh_shell("tofa --version").stdout.strip(), f"tofa {VERSION}")
        self.assertIn("tofa uninstall [--purge]", self.fresh_shell("tofa --help").stdout)
        path = self.fresh_shell('printf "%s" "$PATH"').stdout.split(os.pathsep)
        self.assertEqual(path.count(str(self.binary.parent)), 1)
        self.assertEqual(hashlib.sha256(self.binary.read_bytes()).digest(),
                         hashlib.sha256((DIST / self.asset).read_bytes()).digest())

    def assert_removed(self, purge=False):
        for path in (self.binary, self.install / ".path-files"):
            self.assertFalse(path.exists(), str(path))
        if purge:
            self.assertFalse((self.install / ".tofa-install").exists())
        self.fresh_shell("command -v tofa", success=False)
        path = self.fresh_shell('printf "%s" "$PATH"').stdout.split(os.pathsep)
        self.assertNotIn(str(self.binary.parent), path)
        for rc in self.startup:
            self.assertEqual(rc.read_text().strip(), self.original_rc.strip())
        self.assert_unrelated_state()

    def test_install_repeat_uninstall_reinstall_and_purge(self):
        self.fresh_shell("command -v tofa", success=False)
        self.install_candidate()
        self.assert_available()
        self.assert_saved_state()
        # Removing an owned installation must retain unrelated files even inside it.
        for name in ("unrelated.txt", "bin/other-tool"):
            path = self.install / name
            path.write_bytes(b"keep this file\n")
            self.unrelated[path] = path.read_bytes()
        self.install_candidate()
        self.assert_available()
        for rc in self.startup:
            self.assertEqual(rc.read_text().count("# >>> tofa >>>"), 1)
        self.assertEqual(set((self.install / ".path-files").read_text().splitlines()),
                         {str(rc) for rc in self.startup})
        self.assertEqual(len((self.install / ".path-files").read_text().splitlines()), len(self.startup))
        result = self.fresh_shell("tofa uninstall")
        self.assertIn("Saved preferences and credentials retained", result.stdout)
        self.assert_removed()
        self.assert_saved_state()
        self.install_candidate()
        self.assert_available()
        self.assert_saved_state()
        result = self.fresh_shell("tofa uninstall --purge")
        self.assertIn("Removed tofa and its saved configuration/credentials", result.stdout)
        self.assert_removed(purge=True)
        for path in (*self.saved, self.config / "keyring-refs", self.config / ".auth-lock"):
            self.assertFalse(path.exists(), str(path))

    def test_corrupt_download_preserves_working_installation(self):
        self.install_candidate()
        startup = {rc: rc.read_bytes() for rc in self.startup}
        ownership = (self.install / ".path-files").read_bytes()
        with (self.assets / self.asset).open("ab") as binary:
            binary.write(b"corrupt download")
        result = self.install_candidate(success=False)
        self.assertIn("checksum mismatch", result.stderr)
        self.assertNotIn("Installed tofa", result.stdout)
        self.assert_available()
        self.assert_saved_state()
        self.assert_unrelated_state()
        for rc, content in startup.items():
            self.assertEqual(rc.read_bytes(), content)
        self.assertEqual((self.install / ".path-files").read_bytes(), ownership)

    def test_cli_reports_failed_path_cleanup_and_allows_retry(self):
        self.install_candidate()
        original = self.rc.read_text()
        damaged = original.replace("# <<< tofa <<<", "") + "\n# Keep this trailing comment\n"
        self.rc.write_text(damaged)
        result = self.fresh_shell("tofa uninstall", success=False)
        self.assertIn("damaged tofa PATH markers", result.stderr)
        self.assertNotIn("Removed tofa", result.stdout)
        self.assertEqual(self.rc.read_text(), damaged)
        self.assert_available()
        self.assert_saved_state()
        self.assert_unrelated_state()
        self.rc.write_text(original)
        self.fresh_shell("tofa uninstall")
        self.assert_removed()
        self.assertFalse(self.install.exists())
        self.assert_saved_state()

    def test_cli_reports_blocked_purge_and_preserves_state_for_retry(self):
        self.install_candidate()
        lock = self.config / ".auth-lock"
        lock.mkdir()
        result = self.fresh_shell("tofa uninstall --purge", success=False)
        self.assertIn("authentication operation active", result.stderr)
        self.assertNotIn("Removed tofa", result.stdout)
        self.assert_available()
        self.assert_saved_state()
        self.assert_unrelated_state()
        lock.rmdir()
        self.fresh_shell("tofa uninstall --purge")
        self.assert_removed(purge=True)
        self.assertFalse(self.install.exists())
        for path in self.saved:
            self.assertFalse(path.exists(), str(path))


if __name__ == "__main__":
    if len(sys.argv) < 3:
        raise SystemExit("usage: lifecycle_test.py DIST VERSION [unittest options]")
    DIST = pathlib.Path(sys.argv.pop(1)).resolve()
    VERSION = sys.argv.pop(1)
    if platform.system() not in ("Darwin", "Linux"):
        raise SystemExit("native lifecycle checks require macOS or Linux")
    unittest.main()
