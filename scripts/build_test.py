"""Build/distribution checks using real Go binaries in a disposable source tree."""
import hashlib
import os
import pathlib
import shutil
import subprocess
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parent.parent


class BuildTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="tofa bundle ")
        self.addCleanup(self.temp.cleanup)
        self.source = pathlib.Path(self.temp.name) / "source"
        def ignore(directory, names):
            excluded = {".git", ".agents", ".codex", "dist", "__pycache__"}
            if pathlib.Path(directory) == ROOT:
                excluded.update({"tofa", "tofa.exe"})
            return excluded.intersection(names)
        shutil.copytree(ROOT, self.source, ignore=ignore)

    def build(self, *args):
        return subprocess.run(["sh", "scripts/build.sh", *args], cwd=self.source,
                              text=True, capture_output=True, timeout=240)

    def test_requires_one_explicit_version_before_touching_distribution(self):
        dist = self.source / "dist"
        dist.mkdir()
        sentinel = dist / "existing"
        sentinel.write_text("keep")
        for args in [(), ("latest",), ("../other",), ("v0.1.0-rc.1", "extra")]:
            with self.subTest(args=args):
                result = self.build(*args)
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertEqual(sentinel.read_text(), "keep")

    def test_build_replaces_stale_artifacts_with_six_versioned_binaries(self):
        dist = self.source / "dist"
        dist.mkdir()
        (dist / "tofa_v0.0.0-old_linux_amd64").write_text("stale binary")
        (dist / "obsolete-notice.txt").write_text("stale material")
        version = "v0.1.0-rc.1"
        result = self.build(version)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        binaries = {
            f"tofa_{version}_{system}_{arch}" + (".exe" if system == "windows" else "")
            for system in ("darwin", "linux", "windows") for arch in ("amd64", "arm64")
        }
        self.assertEqual({p.name for p in dist.glob("tofa_*")}, binaries)
        self.assertFalse((dist / "obsolete-notice.txt").exists())
        for name in sorted(binaries):
            info = subprocess.check_output(["go", "version", "-m", str(dist / name)], text=True)
            # Cross-target binaries cannot execute here; check the embedded version,
            # then execute the native binary below to verify the public command.
            self.assertIn(version.encode() + b"\0", (dist / name).read_bytes())
            system, arch = name.removesuffix(".exe").split("_")[-2:]
            self.assertIn(f"GOOS={system}", info)
            self.assertIn(f"GOARCH={arch}", info)
        native_os = subprocess.check_output(["go", "env", "GOHOSTOS"], text=True).strip()
        native_arch = subprocess.check_output(["go", "env", "GOHOSTARCH"], text=True).strip()
        native = dist / f"tofa_{version}_{native_os}_{native_arch}"
        self.assertEqual(subprocess.check_output([str(native), "--version"], text=True).strip(), f"tofa {version}")
        sums = (dist / "SHA256SUMS").read_text().splitlines()
        self.assertEqual(len(sums), len(list(dist.iterdir())) - 1)
        for line in sums:
            digest, name = line.split()
            self.assertEqual(hashlib.sha256((dist / name).read_bytes()).hexdigest(), digest)

    def test_bundle_contains_matching_scripts_and_complete_notices(self):
        result = self.build("v0.1.0-rc.1")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        dist = self.source / "dist"
        for name in ("install.sh", "install.ps1", "uninstall.sh", "uninstall.ps1"):
            self.assertEqual((dist / name).read_bytes(), (self.source / "scripts" / name).read_bytes())
        for name in ("LICENSE", "README.md", "THIRD_PARTY_NOTICES.txt"):
            self.assertEqual((dist / name).read_bytes(), (self.source / name).read_bytes())
        self.assertIn("Permission is hereby granted, free of charge", (dist / "LICENSE").read_text())
        for name in ("LICENSE", "NOTICE"):
            self.assertEqual((dist / f"{name}-CODEX.txt").read_bytes(),
                             (self.source / "internal/tofa/assets" / f"codex-{name}").read_bytes())
        goroot = pathlib.Path(subprocess.check_output(["go", "env", "GOROOT"], text=True).strip())
        self.assertEqual((dist / "LICENSE-GO.txt").read_bytes(), (goroot / "LICENSE").read_bytes())
        notices = (dist / "THIRD_PARTY_NOTICES.txt").read_text()
        modules = set()
        for binary in dist.glob("tofa_*"):
            info = subprocess.check_output(["go", "version", "-m", str(binary)], text=True)
            for line in info.splitlines():
                fields = line.split()
                if fields and fields[0] == "dep":
                    modules.add((fields[1], fields[2]))
        for module, version in modules:
            with self.subTest(module=module):
                self.assertIn(f"{module} {version}", notices)
                directory = pathlib.Path(subprocess.check_output(
                    ["go", "list", "-m", "-f", "{{.Dir}}", module], cwd=self.source, text=True).strip())
                self.assertIn((directory / "LICENSE").read_text(), notices)
                if (directory / "NOTICE").exists():
                    self.assertIn((directory / "NOTICE").read_text(), notices)
        self.assertIn("Copyright 2013 Google Inc. All Rights Reserved.", notices)
        self.assertIn("Apache License\n", notices)
        self.assertIn("END OF TERMS AND CONDITIONS", notices)

    def test_pinned_bundle_installs_from_a_controlled_release_source(self):
        version = "v0.1.0-rc.1"
        result = self.build(version)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        dist = self.source / "dist"
        home = pathlib.Path(self.temp.name) / "home"
        home.mkdir()
        install = home / "install"
        env = dict(os.environ, HOME=str(home), XDG_CONFIG_HOME=str(home / ".config"),
                   TOFA_INSTALL_DIR=str(install), TOFA_RELEASE_BASE_URL=dist.as_uri())
        result = subprocess.run(["sh", str(dist / "install.sh"), "--version", version,
                                 "--no-modify-path"], env=env, text=True, capture_output=True, timeout=30)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(subprocess.check_output([str(install / "bin/tofa"), "--version"],
                                                text=True).strip(), f"tofa {version}")

    def test_failed_build_preserves_previous_distribution(self):
        dist = self.source / "dist"
        dist.mkdir()
        (dist / "existing").write_text("previous bundle")
        # A broken source is a real compiler failure, not a mocked build command.
        (self.source / "cmd/tofa/main.go").write_text("not Go source\n")
        result = self.build("v0.1.0-rc.2")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("expected 'package'", result.stderr)
        self.assertEqual({p.name for p in dist.iterdir()}, {"existing"})
        self.assertEqual((dist / "existing").read_text(), "previous bundle")
        self.assertEqual(list(self.source.glob(".tofa-dist.*")), [])


if __name__ == "__main__":
    unittest.main()
