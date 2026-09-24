"""THROWAWAY #34: qualify a durable executable reference using a shared scratch profile.

python3 internal/tofa/prototype_shared_history/durable_route_probe.py --temporary
python3 internal/tofa/prototype_shared_history/durable_route_probe.py
The temporary variant reproduces the dangling persisted executable; default tests
a durable, credential-free bridge with launch-owned routing. Synthetic inference.
"""
import argparse
import contextlib
import hashlib
import json
import os
from pathlib import Path
import shlex
import shutil
import socket
import subprocess
import sys
import tempfile

import run as prototype


def durable_bridge(root):
    path = root / "bridge/codex"
    path.write_text("#!" + sys.executable + "\n" + '''# THROWAWAY durable bridge: no launch settings or credentials on disk here.
import json, os, sys
engine = ''' + repr(str(prototype.ENGINE)) + '''
env = dict(os.environ)
args = sys.argv[1:]
overrides = []
if "TOFA_DESKTOP_ROUTE" in env:
    try:
        with open(env["TOFA_DESKTOP_ROUTE"]) as source:
            route = json.load(source)
        if not isinstance(route["token"], str) or not route["token"]:
            raise ValueError("missing launch credential")
        if not isinstance(route["overrides"], list) or not all(isinstance(x, str) for x in route["overrides"]):
            raise ValueError("invalid launch settings")
        env["TOFA_PROTOTYPE_KEY"] = route["token"]
        for value in route["overrides"]:
            overrides.extend(["-c", value])
    except (OSError, ValueError, KeyError, TypeError):
        print("PROTOTYPE: tofa launch routing expired or invalid; relaunch through tofa.", file=sys.stderr)
        sys.exit(78)
# app-server owns its -c arguments; root options are displaced if it receives any.
command = [engine, *args, *overrides] if "app-server" in args else [engine, *overrides, *args]
os.execve(engine, command, env)
''')
    path.chmod(0o700)
    return path


@contextlib.contextmanager
def launch(root, bridge, mode, merged, records, routes):
    if mode == "ordinary":
        yield bridge, {}, None
        return
    directory = Path(tempfile.mkdtemp(prefix="launch-", dir=root / "routing"))
    fixture = prototype.Fixture("Token Factory", {prototype.MODEL}, records)
    catalog = directory / "catalog.json"
    catalog.write_text(json.dumps(merged))
    provider = ('{name="Token Factory PROTOTYPE",base_url=' + json.dumps(fixture.endpoint) +
                ',env_key="TOFA_PROTOTYPE_KEY",wire_api="responses",requires_openai_auth=false,'
                'supports_websockets=false,request_max_retries=0,stream_max_retries=0}')
    settings = ['model=' + json.dumps(prototype.MODEL), 'model_provider="nebius-tofa"',
                'model_providers.nebius-tofa=' + provider, 'model_catalog_json=' + json.dumps(str(catalog))]
    manifest = directory / "launch.json"
    manifest.write_text(json.dumps({"token": fixture.token, "overrides": settings}))
    manifest.chmod(0o600)
    routes.append({"endpoint": fixture.endpoint, "token": fixture.token, "manifest": str(manifest)})
    env = {"TOFA_DESKTOP_ROUTE": str(manifest)}
    # Public-RPC driver deliberately has a narrow environment. This invocation
    # script supplies its launch env; only the durable bridge is given to desktop.
    invoke = directory / "invoke"
    invoke.write_text("#!/bin/sh\nexport TOFA_DESKTOP_ROUTE=" + shlex.quote(str(manifest)) +
                      "\nexec " + shlex.quote(str(bridge)) + ' "$@"\n')
    invoke.chmod(0o700)
    try:
        yield invoke, env, manifest
    finally:
        fixture.close()
        shutil.rmtree(directory)


def persisted_executable(root):
    config = (root / "codex/config.toml").read_text()
    section = config.partition("[mcp_servers.node_repl.env]\n")[2].split("\n[", 1)[0]
    matches = [line.split(" = ", 1)[1] for line in section.splitlines() if line.startswith("CODEX_CLI_PATH = ")]
    prototype.check(len(matches) == 1, "desktop tool executable setting not observed")
    return Path(json.loads(matches[0]))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--temporary", action="store_true")
    args = parser.parse_args()
    root = Path(tempfile.mkdtemp(prefix="PROTOTYPE-durable34-", dir="/private/tmp"))
    for name in ("codex", "workspace", "electron", "routing", "shell", "bridge"):
        (root / name).mkdir(mode=0o700)
    (root / "workspace/PROTOTYPE-WIPE-ME.txt").write_text("Synthetic shared-profile routing experiment.\n")
    version = subprocess.check_output([str(prototype.ENGINE), "--version"], env=prototype.scratch_env(root), text=True).strip()
    prototype.check(version == "codex-cli 0.155.0-alpha.16.3", "requalify for the installed engine")
    native_catalog, merged = prototype.catalog(root)
    records, routes = [], []
    native = prototype.Fixture("native", {row["slug"] for row in native_catalog["models"]}, records)
    (root / "codex/auth.json").write_text(json.dumps({"OPENAI_API_KEY": native.token}))
    (root / "codex/auth.json").chmod(0o600)
    (root / "codex/config.toml").write_text(
        'model="' + prototype.NATIVE + '"\nmodel_provider="openai"\nopenai_base_url=' + json.dumps(native.endpoint) +
        '\ncli_auth_credentials_store="file"\napproval_policy="never"\nsandbox_mode="read-only"\n'
        'web_search="disabled"\n[features]\nshell_snapshot=false\nanalytics=false\n')
    auth = prototype.snapshot(root)["auth.json"]
    report = {"issue": 34, "throwaway": True, "engine_version": version,
              "mode": "temporary" if args.temporary else "durable", "real_user_profile_used": False,
              "phases": [], "requests": records}
    report_path = prototype.HERE / ("temporary-reference-evidence.json" if args.temporary else "durable-reference-evidence.json")
    prototype.emit("scratch_profile", path=str(root))
    try:
        if args.temporary:
            with prototype.route(root, "ordinary", native_catalog, merged, records, routes) as wrapper:
                prototype.desktop(root, wrapper, "ordinary", 1, True)
                stored = persisted_executable(root)
            report["persisted_executable_exists_after_cleanup"] = stored.is_file()
            prototype.check(stored.is_file(), "FAIL: desktop persisted executable was deleted by launch cleanup")
        else:
            bridge = durable_bridge(root)
            digest = hashlib.sha256(bridge.read_bytes()).hexdigest()
            threads = {}
            for phase, mode in enumerate(("ordinary", "tofa", "ordinary", "tofa"), 1):
                with launch(root, bridge, mode, merged, records, routes) as (invoke, env, manifest):
                    with contextlib.closing(prototype.Engine(str(invoke), root, [])) as engine:
                        if phase in (1, 2):
                            name = "PROTOTYPE ordinary history" if phase == 1 else "PROTOTYPE tofa history"
                            thread_id = engine.call("thread/start", {"cwd": str(root / "workspace"), "historyMode": "legacy", "threadSource": "user"})["thread"]["id"]
                            engine.turn(thread_id, name)
                            engine.call("thread/name/set", {"threadId": thread_id, "name": name})
                            threads[thread_id] = name
                            if phase == 1:
                                ordinary = thread_id
                            else:
                                adapted = thread_id
                                resumed = engine.call("thread/resume", {"threadId": ordinary})
                                prototype.check(resumed["modelProvider"] == "openai" and resumed["model"] == prototype.NATIVE,
                                                "native model/provider changed")
                                engine.turn(ordinary, "Native continuation during tofa launch")
                        if phase == 3:
                            result = engine.response("thread/resume", {"threadId": adapted})
                            prototype.check("Model provider `nebius-tofa` not found" in result.get("error", {}).get("message", ""),
                                            "ordinary mode unexpectedly retained tofa routing")
                            engine.call("thread/resume", {"threadId": ordinary})
                            engine.turn(ordinary, "Native continuation after tofa cleanup")
                        if phase == 4:
                            resumed = engine.call("thread/resume", {"threadId": adapted})
                            prototype.check(resumed["modelProvider"] == "nebius-tofa" and resumed["model"] == prototype.MODEL,
                                            "tofa relaunch changed recorded provider/model")
                            engine.turn(adapted, "Tofa continuation with fresh launch routing")
                        state = prototype.read_state(engine, threads)
                    desktop = prototype.desktop(root, bridge, mode, phase, True, env)
                    stored = persisted_executable(root)
                    prototype.check(stored == bridge, "desktop persisted an unexpected executable")
                    prototype.check(prototype.snapshot(root)["auth.json"] == auth, "synthetic account credentials changed")
                    config = (root / "codex/config.toml").read_text()
                    prototype.check("TOFA_DESKTOP_ROUTE" not in config and "TOFA_PROTOTYPE_KEY" not in config,
                                    "launch-only environment persisted in shared config")
                    prototype.check(all(row["token"] not in config and row["manifest"] not in config for row in routes),
                                    "launch capability persisted in shared config")
                    report["phases"].append({"phase": phase, "mode": mode, "sessions": state,
                                              "desktop": desktop, "persisted_executable_is_durable_bridge": True})
                prototype.check(bridge.is_file() and hashlib.sha256(bridge.read_bytes()).hexdigest() == digest,
                                "cleanup removed or rewrote durable bridge")
                if manifest:
                    expired_env = dict(prototype.scratch_env(root), TOFA_DESKTOP_ROUTE=str(manifest))
                    stale = subprocess.run([str(stored), "--version"], env=expired_env, capture_output=True, text=True, timeout=5)
                    prototype.check(stale.returncode == 78 and "expired or invalid" in stale.stderr,
                                    "expired launch capability did not fail explicitly")
                    report["phases"][-1]["expired_launch_reference_rejected"] = True
                plain = subprocess.run([str(stored), "--version"], env=prototype.scratch_env(root), capture_output=True, text=True, timeout=5)
                prototype.check(plain.returncode == 0 and plain.stdout.strip() == version,
                                "persisted bridge no longer works outside launch")
                report["phases"][-1]["persisted_executable_works_after_cleanup"] = True
                for expired in routes:
                    with socket.socket() as probe:
                        prototype.check(probe.connect_ex(("127.0.0.1", int(expired["endpoint"].rsplit(":", 1)[1]))) != 0,
                                        "expired tofa listener survived cleanup")
                prototype.emit("phase_passed", phase=phase, mode=mode, durable_reference=True)
            prototype.check(len({row["token"] for row in routes}) == 2, "tofa credentials were reused")
            prototype.check(len({row["endpoint"] for row in routes}) == 2, "tofa endpoints were reused")
            files = [path for path in (root / "codex").rglob("*") if path.is_file()]
            prototype.check(not any(row["token"].encode() in path.read_bytes() for row in routes for path in files),
                            "tofa credential leaked to shared engine state")
            report.update(durable_bridge_unchanged=True, expired_routes_closed=True,
                          tofa_credentials_absent_from_shared_engine_state=True,
                          fresh_tofa_routes=2, ordinary_mode_has_no_tofa_provider=True,
                          shared_config_byte_identical=False, desktop_ui_verified=False,
                          limitations=["scratch profile only", "durable bridge installation, upgrade and uninstall not qualified",
                                       "ordinary-mode tofa history logo loop remains", "no real Token Factory inference",
                                       "unexpected process death not qualified; normal route cleanup and expired references checked"])
        report["result"] = "PASS"
    except BaseException as error:
        report["result"] = "FAIL"
        report["failure"] = str(error)
        raise
    finally:
        native.close()
        report["auth_unchanged"] = prototype.snapshot(root)["auth.json"] == auth
        report_path.write_text(json.dumps(report, indent=2) + "\n")
        prototype.emit("report", path=str(report_path), result=report["result"])


if __name__ == "__main__":
    main()
