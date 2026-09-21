"""Qualify a pinned prerelease in the normal Windows account; reports stay local."""
import argparse
import ctypes
from ctypes import wintypes
import json
import os
from pathlib import Path
import platform
import re
import shutil
import signal
import subprocess
import sys
import tempfile
import threading
import time
from types import SimpleNamespace

import live_compat
from qualify_macos import Failure, STAGES, REPOSITORY, file_state, save_report, vault_refs
from release import prerelease
import windows_process


def command(args, supervisor, timeout, env=None, interactive=False, visible=False, require_descendant_success=False):
    process = subprocess.Popen(windows_process.supervised(args, supervisor, require_descendant_success=require_descendant_success), env=env,
                               stdin=None if interactive else subprocess.DEVNULL,
                               stdout=None if interactive or visible else subprocess.PIPE,
                               stderr=None if interactive or visible else subprocess.PIPE)
    chunks = [bytearray(), bytearray()]
    limited = threading.Event()
    lock = threading.Lock()
    def consume(stream, target):
        while True:
            data = stream.read1(65536)
            if not data: break
            with lock:
                remaining = 1024 * 1024 - sum(map(len, chunks))
                target.extend(data[:remaining])
                if len(data) > remaining:
                    limited.set()
                    if process.poll() is None: process.kill()
                    break
    readers = []
    if not interactive and not visible:
        readers = [threading.Thread(target=consume, args=(stream, target), daemon=True)
                   for stream, target in zip((process.stdout, process.stderr), chunks)]
        for reader in readers: reader.start()
    try:
        process.wait(timeout=timeout)
    except BaseException:
        windows_process.stop(process)
        raise
    finally:
        for reader in readers: reader.join(timeout=10)
        for stream in (process.stdout, process.stderr):
            if stream: stream.close()
    if limited.is_set(): raise Failure("output_limit")
    if process.returncode != 0:
        raise Failure("command_failed")
    return chunks[0].decode("utf-8-sig", errors="replace").strip(), bool(chunks[1])


def confirm(message, answer, timeout):
    import msvcrt
    print(message + f"\nType {answer} to continue: ", end="", flush=True)
    deadline = time.monotonic() + timeout
    response = bytearray()
    console = sys.stdin.isatty()
    while len(response) < 128:
        if time.monotonic() >= deadline:
            raise Failure("human_confirmation_timeout")
        if console:
            if not msvcrt.kbhit():
                time.sleep(0.05); continue
            char = msvcrt.getwch()
            if char == "\x03": raise KeyboardInterrupt
            if char in ("\r", "\n"):
                print(); break
            if char == "\b":
                if response: response.pop(); print("\b \b", end="", flush=True)
                continue
            print(char, end="", flush=True)
            response.extend(char.encode("utf-8"))
        else:
            # select() does not support Windows pipes. Peek before an unbuffered
            # byte read, so later login/gate input is never consumed in advance.
            available = wintypes.DWORD()
            handle = wintypes.HANDLE(msvcrt.get_osfhandle(sys.stdin.fileno()))
            if not ctypes.windll.kernel32.PeekNamedPipe(handle, None, 0, None, ctypes.byref(available), None):
                break
            if not available.value:
                time.sleep(0.05); continue
            char = os.read(sys.stdin.fileno(), 1)
            if char in (b"", b"\n"): break
            response.extend(char)
    if response.decode("utf-8", errors="replace").strip() != answer:
        raise Failure("human_confirmation_declined")


def persistent_path(machine=False):
    import winreg
    hive, key_name = (winreg.HKEY_LOCAL_MACHINE, r"SYSTEM\CurrentControlSet\Control\Session Manager\Environment") if machine else (winreg.HKEY_CURRENT_USER, "Environment")
    with winreg.OpenKey(hive, key_name) as key:
        try: value, _ = winreg.QueryValueEx(key, "Path")
        except FileNotFoundError: return []
    return [entry for entry in value.split(";") if entry]


def unrelated(root, excluded, skip_install=False):
    if not root.exists(): return {}
    result = {}
    for path in root.rglob("*"):
        relative = path.relative_to(root)
        if relative.parts[0] == "keyring-refs" or (skip_install and relative.parts[0] == "install"):
            continue
        if relative.as_posix() not in excluded and (path.is_file() or path.is_symlink()):
            result[relative.as_posix()] = file_state(path)
    return result


def vault_absent(refs):
    # Query only recorded tofa targets. Never dereference credential blobs or
    # enumerate unrelated credentials; immediately free any returned allocation.
    api = ctypes.WinDLL("advapi32", use_last_error=True)
    api.CredReadW.argtypes = [wintypes.LPCWSTR, wintypes.DWORD, wintypes.DWORD, ctypes.POINTER(ctypes.c_void_p)]
    api.CredReadW.restype = wintypes.BOOL
    api.CredFree.argtypes = [ctypes.c_void_p]
    absent = True
    for ref in refs:
        pointer = ctypes.c_void_p()
        if api.CredReadW("io.nebius.tofa.prototype:" + ref, 1, 0, ctypes.byref(pointer)):
            api.CredFree(pointer)
            absent = False
        elif ctypes.get_last_error() != 1168:
            raise Failure("vault_verification_failed")
    return absent


def download(options, directory, supervisor, evidence):
    def run(args): return command(args, supervisor, options.timeout)[0]
    metadata = json.loads(run(["gh", "api", f"repos/{REPOSITORY}/releases/tags/{options.version}"]))
    if metadata.get("tag_name") != options.version or metadata.get("draft") is not False or metadata.get("prerelease") is not True:
        raise Failure("not_a_published_prerelease")
    commit = json.loads(run(["gh", "api", f"repos/{REPOSITORY}/commits/{options.version}"])).get("sha", "")
    if not re.fullmatch(r"[0-9a-f]{40}", commit): raise Failure("invalid_candidate_commit")
    evidence["candidate"]["commit"] = commit
    asset = f"tofa_{options.version}_windows_{windows_process.architecture()}.exe"
    names = (asset, "install.ps1", "uninstall.ps1")
    args = ["gh", "release", "download", options.version, "--repo", REPOSITORY, "--dir", str(directory)]
    for name in (*names, "SHA256SUMS"): args.extend(["--pattern", name])
    run(args)
    sums = {}
    for line in (directory / "SHA256SUMS").read_text().splitlines():
        match = re.fullmatch(r"([0-9a-fA-F]{64})  (\S+)", line)
        if not match or match[2] in sums: raise Failure("invalid_checksums")
        sums[match[2]] = match[1].lower()
    for name in names:
        path = directory / name
        if path.is_symlink() or not path.is_file() or live_compat.digest(path) != sums.get(name):
            raise Failure("checksum_mismatch")
    evidence["candidate"].update(binary_sha256=sums[asset], scripts_sha256={name: sums[name] for name in names[1:]})
    return asset


def qualify(options, reports):
    config = Path(os.environ.get("LOCALAPPDATA", "")) / "tofa"
    install = Path(os.environ.get("TOFA_INSTALL_DIR") or config / "install")
    binary = install / "bin/tofa.exe"
    client_home = Path(os.environ.get("CODEX_HOME", str(Path.home() / ".codex")))
    watched = {"codex_config": client_home / "config.toml", "codex_auth": client_home / "auth.json"}
    config_owned = {"config.yml", "credentials.yml", ".auth-lock"}
    install_owned = {"bin/tofa.exe", ".path-owned", ".tofa-install"}
    evidence = {"schema_version": 1, "candidate": {"tag": options.version},
                "host": {"os": platform.system(), "arch": platform.machine(), "release": platform.version()},
                "client": {}, "backend": "unknown", "model": live_compat.MODEL,
                "started_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
                "stages": [{"name": name, "status": "pending"} for name in STAGES],
                "preservation": {name: None for name in ("baseline_recorded", "codex_config", "codex_auth", "unrelated_config", "unrelated_installation", "user_path", "machine_path")},
                "cleanup": {name: None for name in ("scratch_removed", "binary_removed", "ownership_removed", "path_entries_removed", "configuration_removed", "file_credentials_removed", "vault_credentials_removed", "helpers_completed")},
                "outcome": "incomplete"}
    active, scratch, supervisor = None, None, None
    before, config_before, install_before = {}, {}, {}
    refs, user_before, machine_before = set(), [], []
    baseline = False
    helpers_completed = True
    bin_path = str(binary.parent)
    default_install_covered = install.resolve() == (config / "install").resolve()

    def stage(name, action):
        nonlocal active
        active = next(item for item in evidence["stages"] if item["name"] == name)
        active["status"] = "running"
        print("Stage: " + name, flush=True)
        result = action()
        active["status"] = "passed"
        return result

    def run(args, **kwargs): return command(args, supervisor, options.timeout, **kwargs)[0]

    def preflight():
        nonlocal before, config_before, install_before, refs, user_before, machine_before, baseline, supervisor
        if os.name != "nt" or not config.is_absolute(): raise Failure("unsupported_platform")
        before = {name: file_state(path) for name, path in watched.items()}
        config_before = unrelated(config, config_owned, skip_install=default_install_covered)
        install_before = unrelated(install, install_owned)
        user_before = persistent_path()
        machine_before = persistent_path(True)
        baseline = True
        if config.is_symlink() or install.is_symlink() or (config / ".auth-lock").exists():
            raise Failure("unsafe_or_busy_existing_state")
        if bin_path.lower() in [entry.lower() for entry in user_before] and not (install / ".path-owned").is_file():
            raise Failure("unowned_installation_path")
        for program in ("gh", "powershell.exe"):
            if not shutil.which(program): raise Failure("missing_prerequisite")
        options.codex = windows_process.resolve_codex(options.codex)
        supervisor = windows_process.build_supervisor(Path(scratch.name))
        refs = vault_refs(config)
        evidence["existing_state"] = {"installation": install.exists(), "configuration": (config / "config.yml").exists(),
                                      "file_credentials": (config / "credentials.yml").exists(), "vault_references": bool(refs)}
        probe = Path(scratch.name) / "probe"
        probe.mkdir()
        env = live_compat.client_environment(probe, probe / "codex")
        version = run(options.codex + ["--version"], env=env)
        if not re.fullmatch(r"codex-cli [0-9]+(?:\.[0-9]+)+(?:[-+][0-9A-Za-z.-]+)?", version):
            raise Failure("unrecognized_client_version")
        evidence["client"]["version"] = version

    def discovery(removed=False):
        persistent = persistent_path(True) + persistent_path()
        env = dict(os.environ, PATH=os.path.expandvars(";".join(persistent)))
        found = shutil.which("tofa.exe", path=env["PATH"])
        if removed:
            if any(entry.lower() == bin_path.lower() for entry in persistent) or (found and Path(found) == binary):
                raise Failure("persistent_path_cleanup_incomplete")
        else:
            if not found or Path(found).resolve() != binary.resolve() or sum(entry.lower() == bin_path.lower() for entry in persistent_path()) != 1:
                raise Failure("persistent_path_discovery_failed")
            if run([found, "--version"], env=env) != "tofa " + options.version:
                raise Failure("persistent_path_version_mismatch")

    def install_candidate():
        run(["powershell.exe", "-NoProfile", "-File", str(Path(__file__).with_name("qualify_windows_install.ps1")),
             "-Directory", str(downloads), "-Version", options.version, "-Asset", asset], visible=True)
        if live_compat.digest(binary) != evidence["candidate"]["binary_sha256"]: raise Failure("installed_checksum_mismatch")
        if run([str(binary), "--version"]) != "tofa " + options.version: raise Failure("installed_version_mismatch")
        discovery()

    def login():
        nonlocal refs
        print("Enter Token Factory credentials in the launcher. Respond to any OS prompts.", flush=True)
        args = [str(binary), "auth", "login"]
        if options.storage: args += ["--storage", options.storage]
        command(args, supervisor, options.human_timeout, interactive=True)
        match = re.search(r"^Credential backend: (file|keyring) \(", run([str(binary), "doctor"]), re.M)
        if not match: raise Failure("credential_backend_unknown")
        evidence["backend"] = match[1]
        refs |= vault_refs(config)
        if evidence["backend"] == "keyring" and (not refs or vault_absent(refs)):
            raise Failure("saved_vault_credential_missing")

    def live(name):
        with tempfile.TemporaryDirectory(prefix="live-", dir=scratch.name) as directory:
            result = live_compat.run_one(SimpleNamespace(launcher=str(binary), codex=options.codex, timeout=options.timeout,
                                                       supervisor=supervisor), Path(directory).resolve())
        evidence[name] = result
        if any(before[name] != file_state(path) for name, path in watched.items()): raise Failure("preservation_failed")
        if not result["passed"]: raise Failure("live_assertions_failed")

    def uninstall(purge=False):
        nonlocal helpers_completed
        if purge:
            confirm("Purge removes local tofa preferences and credentials, without revoking remote tokens. Confirm recovery is ready.", "PURGE", options.human_timeout)
        saved = {name: file_state(config / name) for name in ("config.yml", "credentials.yml")}
        saved_refs = vault_refs(config)
        helpers_completed = False
        helpers = Path(scratch.name) / ("purge-helpers" if purge else "preserve-helpers")
        helpers.mkdir()
        args = [str(binary), "uninstall"] + (["--purge"] if purge else [])
        try:
            output, errors = command(args, supervisor, options.timeout, env=dict(os.environ, TEMP=str(helpers), TMP=str(helpers)), require_descendant_success=True)
        except Failure as error:
            if str(error) == "command_failed": raise Failure("uninstall_helper_failed") from None
            raise
        # The supervisor waits for every child in the Job, including detached
        # CLI helpers. Process exit alone cannot establish successful cleanup.
        if errors or "Uninstaller started;" not in output or "Existing terminals may retain the old PATH entry." not in output or list(helpers.iterdir()):
            raise Failure("uninstall_helper_failed")
        helpers_completed = True
        if binary.exists() or (install / ".path-owned").exists(): raise Failure("uninstall_cleanup_incomplete")
        discovery(removed=True)
        if not purge and (saved != {name: file_state(config / name) for name in saved} or saved_refs != vault_refs(config)):
            raise Failure("saved_state_changed")
        if purge and not vault_absent(refs): raise Failure("vault_cleanup_incomplete")

    try:
        scratch = tempfile.TemporaryDirectory(prefix="tofa-qualification-")
        downloads = Path(scratch.name) / "download"
        downloads.mkdir()
        stage("preflight", preflight)
        stage("recovery", lambda: confirm("This run replaces launcher/login, then preserves, reinstalls and explicitly purges local tofa state. Existing state: " + json.dumps(evidence["existing_state"]) + ". Prepare credential recovery yourself and close other sessions.", "READY", options.human_timeout))
        asset = stage("download", lambda: download(options, downloads, supervisor, evidence))
        stage("install", install_candidate)
        stage("login", login)
        stage("fresh_terminal", lambda: confirm("Open a NEW terminal. Run `Get-Command tofa` and `tofa --version`; confirm " + str(binary) + " and tofa " + options.version + ". Record any OS prompts separately.", "FOUND", options.human_timeout))
        stage("live", lambda: live("live"))
        stage("uninstall", uninstall)
        stage("reinstall", install_candidate)
        stage("saved_login_reuse", lambda: live("saved_login_reuse"))
        stage("purge", lambda: uninstall(True))
        evidence["outcome"] = "passed"
    except (KeyboardInterrupt, EOFError):
        evidence["reason"] = "interrupted"
    except Failure as error:
        evidence.update(outcome="incomplete" if str(error).startswith("human_confirmation") else "failed", reason=str(error))
    except subprocess.TimeoutExpired:
        evidence.update(outcome="failed", reason="process_timeout")
    except (OSError, ValueError, KeyError, TypeError, RuntimeError, subprocess.SubprocessError):
        evidence.update(outcome="failed", reason="operation_failed")
    finally:
        if active and active["status"] == "running": active["status"] = evidence["outcome"]
        try:
            if scratch: scratch.cleanup()
            evidence["cleanup"].update(scratch_removed=True, binary_removed=not binary.exists(),
                ownership_removed=not (install / ".tofa-install").exists(),
                path_entries_removed=not (install / ".path-owned").exists() and all(entry.lower() != bin_path.lower() for entry in persistent_path()),
                configuration_removed=not (config / "config.yml").exists(), file_credentials_removed=not (config / "credentials.yml").exists(),
                vault_credentials_removed=vault_absent(refs) and not vault_refs(config), helpers_completed=helpers_completed)
            evidence["preservation"].update(baseline_recorded=baseline,
                **{name: before.get(name) == file_state(path) for name, path in watched.items()},
                unrelated_config=config_before == unrelated(config, config_owned, skip_install=default_install_covered),
                unrelated_installation=install_before == unrelated(install, install_owned),
                user_path=[p for p in user_before if p.lower() != bin_path.lower()] == [p for p in persistent_path() if p.lower() != bin_path.lower()],
                machine_path=machine_before == persistent_path(True))
            if evidence["outcome"] == "passed" and not all(evidence["cleanup"].values()): evidence.update(outcome="failed", reason="cleanup_incomplete")
            if not all(evidence["preservation"].values()): evidence.update(outcome="failed", reason="preservation_failed")
        except (OSError, ValueError, Failure):
            evidence.update(outcome="failed", reason="final_checks_failed")
        try: save_report(reports, evidence)
        except OSError:
            print("Could not finish local reports; inspect remaining state before rerunning.", file=sys.stderr)
            evidence["outcome"] = "failed"
        else: print("Qualification " + evidence["outcome"] + ". Local reports saved; no upload performed.", flush=True)
    return 0 if evidence["outcome"] == "passed" else 1


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--version", required=True, type=prerelease)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--codex", default=shutil.which("codex"))
    parser.add_argument("--storage", choices=("file", "keyring"))
    parser.add_argument("--timeout", type=int, default=180)
    parser.add_argument("--human-timeout", type=int, default=900)
    options = parser.parse_args()
    if os.name != "nt": parser.error("native Windows is required")
    options.output = options.output.resolve()
    if options.output.suffix != ".json" or not options.output.parent.is_dir() or any(p.exists() for p in (options.output, options.output.with_suffix(".md"))):
        parser.error("output requires new .json and .md files in an existing directory")
    if not 1 <= options.timeout <= 600 or not 1 <= options.human_timeout <= 3600: parser.error("invalid timeout")
    reports, reserved = [], []
    try:
        for path in (options.output, options.output.with_suffix(".md")):
            fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
            reserved.append(path)
            reports.append(os.fdopen(fd, "w", encoding="utf-8"))
    except OSError:
        for stream in reports: stream.close()
        for path in reserved: path.unlink()
        parser.error("cannot create reports; account state was not changed")
    def interrupt(*unused): raise KeyboardInterrupt
    signal.signal(signal.SIGTERM, interrupt)
    signal.signal(signal.SIGBREAK, interrupt)
    with reports[0], reports[1]: return qualify(options, reports)


if __name__ == "__main__":
    sys.exit(main())
