"""Native executable-boundary tests for Windows qualification process ownership."""

import json
import ctypes
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import shutil
import unittest
from unittest.mock import patch

import windows_process


class CodexResolutionTests(unittest.TestCase):
    def test_architecture_uses_native_os_over_emulated_process_architecture(self):
        for environment, expected in [
            ({"PROCESSOR_ARCHITECTURE": "AMD64"}, "amd64"),
            ({"PROCESSOR_ARCHITECTURE": "ARM64"}, "arm64"),
            ({"PROCESSOR_ARCHITECTURE": "x86", "PROCESSOR_ARCHITEW6432": "AMD64"}, "amd64"),
            ({"PROCESSOR_ARCHITECTURE": "AMD64", "PROCESSOR_ARCHITEW6432": "ARM64"}, "arm64"),
        ]:
            with self.subTest(environment=environment), patch.dict(os.environ, environment, clear=True):
                self.assertEqual(windows_process.architecture(), expected)
        for environment in [{}, {"PROCESSOR_ARCHITECTURE": "x86"}]:
            with self.subTest(environment=environment), patch.dict(os.environ, environment, clear=True):
                with self.assertRaises(ValueError):
                    windows_process.architecture()

    def test_resolves_native_executable_and_npm_wrapper_without_a_shell(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            native = root / "codex.exe"
            native.touch()
            self.assertEqual(windows_process.resolve_codex(str(native)), [str(native)])
            wrapper = root / "codex.cmd"
            wrapper.write_text("@echo off", encoding="utf-8")
            script = root / "node_modules" / "@openai" / "codex" / "bin" / "codex.js"
            script.parent.mkdir(parents=True)
            script.touch()
            node = root / "node.exe"
            node.touch()
            self.assertEqual(windows_process.resolve_codex(str(wrapper)), [str(node), str(script)])
            script.unlink()
            with self.assertRaises(ValueError):
                windows_process.resolve_codex(str(wrapper))
            unsupported = root / "custom.cmd"
            unsupported.touch()
            with self.assertRaises(ValueError):
                windows_process.resolve_codex(str(unsupported))


@unittest.skipUnless(os.name == "nt", "requires native Windows Job Objects")
class WindowsProcessTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.directory = tempfile.TemporaryDirectory(prefix="tofa process ")
        cls.root = Path(cls.directory.name)
        cls.supervisor = windows_process.build_supervisor(cls.root)

    @classmethod
    def tearDownClass(cls):
        cls.directory.cleanup()

    def test_preserves_arguments_environment_and_exit_status(self):
        arguments = ["", "space value", '{"model":"a b"}', 'quote"value', "trailing\\", "a\\\\\"b", "雪"]
        code = "import json,os,sys; print(json.dumps([sys.argv[1:],os.environ['TOFA_TEST_VALUE'],sys.stdin.read()])); sys.exit(17)"
        result = subprocess.run(
            windows_process.supervised([sys.executable, "-c", code, *arguments], self.supervisor),
            env=dict(os.environ, TOFA_TEST_VALUE="preserved value"),
            capture_output=True, text=True, input="unchanged stdin\n", timeout=20,
        )
        self.assertEqual(result.returncode, 17)
        self.assertEqual(json.loads(result.stdout), [arguments, "preserved value", "unchanged stdin\n"])

    def test_native_codex_shim_preserves_observed_arguments(self):
        shim = self.root / "codex.exe"
        shutil.copy2(self.supervisor, shim)
        observer = self.root / "observer.py"
        observer.write_text("import json,sys; print(json.dumps(sys.argv[1:]))", encoding="utf-8")
        result = subprocess.run(
            [str(shim), "-c", '{"key":"quoted value"}'],
            env=dict(os.environ, TOFA_LIVE_PYTHON=sys.executable, TOFA_LIVE_HARNESS=str(observer)),
            capture_output=True, text=True, timeout=20,
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(json.loads(result.stdout), ["--observe", "-c", '{"key":"quoted value"}'])

    def test_waits_for_async_descendants_after_command_exit(self):
        marker = self.root / "async-finished"
        child = "import time,pathlib; time.sleep(0.4); pathlib.Path(" + repr(str(marker)) + ").touch()"
        parent = "import subprocess,sys; subprocess.Popen([sys.executable,'-c'," + repr(child) + "],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)"
        result = subprocess.run(windows_process.supervised([sys.executable, "-c", parent], self.supervisor), timeout=20)
        self.assertEqual(result.returncode, 0)
        self.assertTrue(marker.is_file())

    def test_timeout_cancellation_kills_descendants(self):
        child = "import os,time; print(os.getpid(),flush=True); time.sleep(60)"
        parent = "import subprocess,sys; subprocess.Popen([sys.executable,'-c'," + repr(child) + "])"
        process = subprocess.Popen(
            windows_process.supervised([sys.executable, "-c", parent], self.supervisor),
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True,
        )
        kernel = ctypes.WinDLL("kernel32", use_last_error=True)
        kernel.OpenProcess.restype = ctypes.c_void_p
        kernel.OpenProcess.argtypes = [ctypes.c_uint32, ctypes.c_int, ctypes.c_uint32]
        kernel.WaitForSingleObject.argtypes = [ctypes.c_void_p, ctypes.c_uint32]
        kernel.CloseHandle.argtypes = [ctypes.c_void_p]
        handle = None
        try:
            pid = int(process.stdout.readline())
            handle = kernel.OpenProcess(0x100000, False, pid)
            self.assertTrue(handle)
            with self.assertRaises(subprocess.TimeoutExpired):
                process.wait(timeout=0.2)
            windows_process.stop(process)
            self.assertEqual(kernel.WaitForSingleObject(handle, 5000), 0)
        finally:
            windows_process.stop(process)
            process.communicate(timeout=10)
            if handle:
                kernel.CloseHandle(handle)

    def test_spawn_failure_has_fixed_diagnostic(self):
        result = subprocess.run(
            windows_process.supervised([str(self.root / "missing-secret-value.exe")], self.supervisor),
            capture_output=True, text=True, timeout=20,
        )
        self.assertEqual(result.returncode, 125)
        self.assertEqual(result.stderr.strip(), "Windows process supervisor failed (125).")


if __name__ == "__main__":
    unittest.main()
