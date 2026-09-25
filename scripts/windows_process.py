"""Native Windows process ownership, without shell command parsing."""

import ctypes
from ctypes import wintypes
import os
from pathlib import Path
import shutil
import subprocess


def architecture():
    """Return the native OS release architecture, including emulated Python."""
    # Environment variables describe the emulated process on Windows ARM64.
    # IsWow64Process2 reports the host independently of the caller's architecture.
    kernel = ctypes.WinDLL("kernel32", use_last_error=True)
    query = kernel.IsWow64Process2
    query.argtypes = [wintypes.HANDLE, ctypes.POINTER(wintypes.USHORT), ctypes.POINTER(wintypes.USHORT)]
    query.restype = wintypes.BOOL
    process_machine, native_machine = wintypes.USHORT(), wintypes.USHORT()
    if not query(wintypes.HANDLE(-1), ctypes.byref(process_machine), ctypes.byref(native_machine)):
        raise ctypes.WinError()
    result = {0x8664: "amd64", 0xAA64: "arm64"}.get(native_machine.value)
    if result is None:
        raise ValueError("Unsupported native Windows architecture: " + hex(native_machine.value))
    return result


def resolve_codex(value):
    """Resolve native Codex or the standard npm shim to shell-free argv."""
    candidate = Path(value).expanduser()
    if not candidate.is_file():
        found = shutil.which(str(value))
        if not found:
            raise ValueError("Codex executable was not found")
        candidate = Path(found)
    candidate = candidate.resolve()
    if candidate.suffix.lower() == ".exe":
        return [str(candidate)]
    if candidate.name.lower() != "codex.cmd":
        raise ValueError("Codex must be a native executable or standard npm codex.cmd")
    script = candidate.parent / "node_modules" / "@openai" / "codex" / "bin" / "codex.js"
    if not script.is_file():
        raise ValueError("The npm Codex JavaScript entrypoint is missing")
    node = candidate.parent / "node.exe"
    if not node.is_file():
        found = shutil.which("node.exe")
        if not found:
            raise ValueError("The npm Codex wrapper requires node.exe")
        node = Path(found).resolve()
    return [str(node), str(script)]


def build_supervisor(directory):
    """Compile the checked-in helper with Windows' installed .NET compiler."""
    if os.name != "nt":
        raise RuntimeError("Windows process supervision requires native Windows")
    windows = Path(os.environ.get("SystemRoot", r"C:\Windows"))
    compilers = [windows / "Microsoft.NET" / folder / "v4.0.30319" / "csc.exe"
                 for folder in ("Framework64", "Framework")]
    compiler = next((path for path in compilers if path.is_file()), None)
    if compiler is None:
        raise RuntimeError("Windows .NET Framework C# compiler is required")
    output = Path(directory) / "tofa-supervisor.exe"
    output.parent.mkdir(parents=True, exist_ok=True)
    result = subprocess.run(
        [str(compiler), "/nologo", "/target:exe", "/out:" + str(output),
         str(Path(__file__).with_suffix(".cs"))],
        stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=60,
    )
    if result.returncode or not output.is_file():
        raise RuntimeError("Windows process supervisor compilation failed")
    return output


def supervised(args, supervisor, require_descendant_success=False):
    """Own the command tree; optionally require every cleanup process to succeed."""
    if not args:
        raise ValueError("A supervised command is required")
    flags = ["--require-descendant-success"] if require_descendant_success else []
    return [str(supervisor), *flags, *map(str, args)]


def stop(process):
    """Terminate a supervisor; closing its Job Object kills its descendants."""
    if process.poll() is None:
        process.kill()
    process.wait(timeout=10)
