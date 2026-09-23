"""THROWAWAY #34 prototype. Only synthetic credentials and an owned scratch profile.

python3 internal/tofa/prototype_shared_history/run.py
Add --desktop for a human-operated, four-phase run of the installed desktop.
Add --desktop-smoke for startup/transport checks without claiming UI observation.
No real inference, ordinary profile access, or installed-client modification.
"""

import argparse
import contextlib
import hashlib
import http.server
import json
import os
from pathlib import Path
import secrets
import shlex
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import threading
import time

REPO = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(REPO / "scripts"))
from desktop_history_test import Engine  # Existing public-RPC driver, not a test run.

BUNDLE = Path("/Applications/ChatGPT.app")
ENGINE = BUNDLE / "Contents/Resources/codex"
MODEL = "moonshotai/Kimi-K3"
NATIVE = "gpt-6-astra"
HERE = Path(__file__).resolve().parent
PROBE = "printf '\\0%s\\0' '_SHELL_ENV_DELIMITER_'; command env -0 || exit; printf '\\0%s\\0' '_SHELL_ENV_DELIMITER_'; exit"


def emit(stage, **state):
    print(json.dumps({"stage": stage, **state}), flush=True)


def check(condition, message):
    if not condition:
        raise RuntimeError(message)


class Fixture:
    def __init__(self, lane, allowed, records, *, token=None, port=0):
        self.token = token or secrets.token_hex(32)
        token = self.token

        class Handler(http.server.BaseHTTPRequestHandler):
            def log_message(self, *_):
                pass

            def do_POST(self):
                payload = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                model = payload.get("model")
                metadata = payload.get("client_metadata", {})
                try:
                    source = json.loads(metadata.get("x-codex-turn-metadata", "{}"))
                except (ValueError, TypeError):
                    source = {}
                authenticated = self.headers.get("Authorization") == "Bearer " + token
                status = 200 if authenticated and model in allowed and self.path == "/responses" else 400
                row = {"lane": lane, "model": model, "path": self.path, "status": status,
                       "authenticated": authenticated, "source": source.get("thread_source")}
                records.append(row)
                emit("request", **row)
                if status != 200:
                    self.send_response(status)
                    self.end_headers()
                    self.wfile.write(b'{"error":{"message":"PROTOTYPE: model, route, or credential not allowed"}}')
                    return
                answer = "PROTOTYPE reply from " + lane + ". Shared history remains the same conversation."
                if source.get("thread_source") == "thread_title":
                    answer = json.dumps({"title": "Prototype shared history", "description": "Synthetic profile experiment"})
                item = {"id": "msg_" + secrets.token_hex(4), "type": "message", "role": "assistant", "status": "completed",
                        "content": [{"type": "output_text", "text": answer, "annotations": []}]}
                response = {"id": "resp_" + secrets.token_hex(4), "status": "completed", "output": [item],
                            "usage": {"input_tokens": 10, "output_tokens": 10, "total_tokens": 20}}
                events = [("response.created", {"response": {"id": response["id"], "status": "in_progress", "output": []}}),
                          ("response.output_item.added", {"output_index": 0, "item": item}),
                          ("response.output_item.done", {"output_index": 0, "item": item}),
                          ("response.completed", {"response": response})]
                body = "".join("event: " + kind + "\ndata: " + json.dumps(dict(value, type=kind)) + "\n\n"
                               for kind, value in events).encode()
                self.send_response(200)
                self.send_header("Content-Type", "text/event-stream")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

        self.server = http.server.ThreadingHTTPServer(("127.0.0.1", port), Handler)
        self.port = self.server.server_port
        self.endpoint = "http://127.0.0.1:" + str(self.port)
        self.worker = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.worker.start()

    def close(self):
        self.server.shutdown()
        self.server.server_close()
        self.worker.join(timeout=5)
        with socket.socket() as probe:
            check(probe.connect_ex(("127.0.0.1", self.port)) != 0, "fixture listener survived close")


def scratch_env(root):
    env = {key: os.environ[key] for key in ("PATH", "TMPDIR", "USER", "LOGNAME", "LANG") if key in os.environ}
    env.update(HOME=str(root), CODEX_HOME=str(root / "codex"), SHELL="/bin/zsh", OTEL_SDK_DISABLED="true")
    return env


def catalog(root):
    result = subprocess.run([str(ENGINE), "debug", "models", "--bundled"], env=scratch_env(root),
                            cwd=root / "workspace", capture_output=True, text=True, timeout=15, check=True)
    native = json.loads(result.stdout)
    snapshot = json.loads((REPO / "internal/tofa/assets/evaluation-candidates.json").read_text())["models"][MODEL]
    kimi = {"slug": MODEL, "display_name": "Kimi-K3 (Token Factory PROTOTYPE)",
            "description": "Synthetic responses only", "supported_reasoning_levels": [],
            "shell_type": "unified_exec", "visibility": "list", "supported_in_api": True,
            "priority": -1, "include_apps_usage_instructions": False,
            "supports_reasoning_summary_parameter": False, "support_verbosity": False,
            "truncation_policy": {"mode": "bytes", "limit": 10000},
            "context_window": snapshot["context_window"], "max_context_window": snapshot["context_window"],
            "effective_context_window_percent": 95, "experimental_supported_tools": [],
            "input_modalities": snapshot["input_modalities"],
            "model_messages": {"instructions_template": (REPO / "internal/tofa/assets/codex-prompt.md").read_text()}}
    return native, {"models": [kimi, *native["models"]]}


@contextlib.contextmanager
def route(root, mode, native_catalog, merged, records, routes):
    directory = Path(tempfile.mkdtemp(prefix="PROTOTYPE-route-", dir=root / "routing"))
    fixture = Fixture("Token Factory", {MODEL}, records) if mode == "tofa" else None
    overrides = []
    if fixture:
        path = directory / "catalog.json"
        path.write_text(json.dumps(merged))
        provider = ('{name="Token Factory PROTOTYPE",base_url=' + json.dumps(fixture.endpoint) +
                    ',env_key="TOFA_PROTOTYPE_KEY",wire_api="responses",requires_openai_auth=false,'
                    'supports_websockets=false,request_max_retries=0,stream_max_retries=0}')
        overrides = ['model=' + json.dumps(MODEL), 'model_provider="nebius-tofa"',
                     'model_providers.nebius-tofa=' + provider, 'model_catalog_json=' + json.dumps(str(path))]
        routes.append({"endpoint": fixture.endpoint, "token": fixture.token})
    args = [str(ENGINE)]
    for override in overrides:
        args.extend(["-c", override])
    wrapper = directory / "codex"
    # app-server has its own config overrides. When present, they displace the
    # root CLI overrides, so inject launch settings at the subcommand boundary.
    app_server_exec = ("for arg do\n  if [ \"$arg\" = app-server ]; then\n    exec " +
                       shlex.quote(str(ENGINE)) + ' "$@" ' + shlex.join(args[1:]) +
                       "\n  fi\ndone\n") if overrides else ""
    wrapper.write_text("#!/bin/sh\n# THROWAWAY routing; never written into the shared profile.\n" +
                       ("export TOFA_PROTOTYPE_KEY=" + shlex.quote(fixture.token) + "\n" if fixture else "") +
                       app_server_exec +
                       "exec " + shlex.join(args) + ' "$@"\n')
    wrapper.chmod(0o700)
    try:
        if mode == "tofa":
            dumped = subprocess.run([str(wrapper), "debug", "models"], env=scratch_env(root),
                                    cwd=root / "workspace", capture_output=True, text=True, timeout=15, check=True)
            actual = {model["slug"]: model for model in json.loads(dumped.stdout)["models"]}
            check(all(actual[model["slug"]] == model for model in native_catalog["models"]),
                  "merged catalog changed a bundled native descriptor")
        emit("route_ready", mode=mode, temporary_provider=bool(fixture), native_descriptors_preserved=True)
        yield wrapper
    finally:
        if fixture:
            fixture.close()
        shutil.rmtree(directory)
        emit("route_closed", mode=mode, temporary_files_removed=True, listener_closed=True)


def snapshot(root):
    return {name: hashlib.sha256((root / "codex" / name).read_bytes()).hexdigest()
            for name in ("config.toml", "auth.json")}


def read_state(engine, threads):
    listed = engine.call("thread/list", {"modelProviders": [], "useStateDbOnly": True})["data"]
    for thread_id in threads:
        check(sum(row["id"] == thread_id for row in listed) == 1, "missing or duplicate prototype thread")
    result = []
    for thread_id in threads:
        thread = engine.call("thread/read", {"threadId": thread_id, "includeTurns": True})["thread"]
        result.append({"label": threads[thread_id], "provider": thread["modelProvider"],
                       "title": thread["name"], "turns": len(thread["turns"]),
                       "workspace_matches": Path(thread["cwd"]).name == "workspace"})
    return result


def desktop(root, wrapper, mode, phase, smoke):
    env = scratch_env(root)
    env.update(CODEX_CLI_PATH=str(wrapper), CODEX_ELECTRON_USER_DATA_PATH=str(root / "electron"),
               ZDOTDIR=str(root / "shell"))
    # Reproduce the pinned launcher's exact environment-query guard, using only
    # empty scratch shell startup for this experiment.
    (root / "shell/.zshenv").write_text(
        "if [[ ${CODEX_SHELL:-} == 1 && ${ZSH_EXECUTION_STRING:-} == " + shlex.quote(PROBE) + " ]]; then\n"
        "  printf '\\0%s\\0' '_SHELL_ENV_DELIMITER_'\n  /usr/bin/env -0\n"
        "  printf '\\0%s\\0' '_SHELL_ENV_DELIMITER_'\n  exit\nfi\n")
    with (root / ("desktop-" + str(phase) + ".log")).open("wb") as log:
        process = subprocess.Popen([str(BUNDLE / "Contents/MacOS/ChatGPT"), "--user-data-dir=" + str(root / "electron"),
                                    "codex://threads/new?mode=codex"], env=env, cwd=root / "workspace",
                                   stdout=log, stderr=log, start_new_session=True)
        try:
            emit("desktop_open", phase=phase, mode=mode, owned_pid=process.pid,
                 instruction="Observing startup only; no UI claim." if smoke else
                 "Inspect available PROTOTYPE sessions and send a harmless continuation. Then press Enter in this runner.")
            if smoke:
                time.sleep(5)
            else:
                input()
            check(process.poll() is None, "owned desktop exited before observation")
            log_text = (root / ("desktop-" + str(phase) + ".log")).read_text()
            check("outcome=success transportKind=stdio" in log_text, "desktop engine handshake not observed")
            check("executablePath=" + str(wrapper) in log_text, "desktop did not select the temporary executable")
            check("listen EINVAL" not in log_text, "desktop IPC socket failed")
        finally:
            if process.poll() is None:
                os.killpg(process.pid, signal.SIGTERM)
                try:
                    process.wait(timeout=8)
                except subprocess.TimeoutExpired:
                    os.killpg(process.pid, signal.SIGKILL)
                    process.wait(timeout=5)
            # Only the owned session's process group is eligible for cleanup.
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            emit("desktop_closed", phase=phase, mode=mode)
    return {"phase": phase, "mode": mode, "temporary_executable_selected": True,
            "engine_handshake_succeeded": True, "ipc_path_error": False,
            "ui_verified": False, "owned_process_group_stopped": True}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--desktop", action="store_true")
    parser.add_argument("--desktop-smoke", action="store_true")
    parser.add_argument("--report", type=Path, default=HERE / "evidence.json")
    args = parser.parse_args()
    # macOS Unix-domain socket paths have a small length limit. The default
    # per-user temporary directory plus the desktop's IPC suffix exceeds it.
    root = Path(tempfile.mkdtemp(prefix="PROTOTYPE-tofa34-", dir="/private/tmp")).resolve()
    for name in ("codex", "workspace", "electron", "routing", "shell"):
        (root / name).mkdir(mode=0o700)
    (root / "workspace/PROTOTYPE-WIPE-ME.txt").write_text("Only synthetic sessions. No production workspace.\n")
    open_desktop = args.desktop or args.desktop_smoke
    emit("scratch_profile", path=str(root), desktop=open_desktop)
    version = subprocess.check_output([str(ENGINE), "--version"], env=scratch_env(root), text=True, timeout=5).strip()
    check(version == "codex-cli 0.155.0-alpha.9.2", "requalify this prototype for the installed engine")
    native_catalog, merged = catalog(root)
    records, routes, phases, threads, desktop_phases = [], [], [], {}, []
    native = Fixture("native", {model["slug"] for model in native_catalog["models"]}, records)
    (root / "codex/auth.json").write_text(json.dumps({"OPENAI_API_KEY": native.token}))
    (root / "codex/auth.json").chmod(0o600)
    (root / "codex/config.toml").write_text(
        'model = "' + NATIVE + '"\nmodel_provider = "openai"\nopenai_base_url = ' + json.dumps(native.endpoint) +
        '\ncli_auth_credentials_store = "file"\napproval_policy = "never"\nsandbox_mode = "read-only"\n'
        'web_search = "disabled"\n[features]\nshell_snapshot = false\nanalytics = false\n')
    baseline = snapshot(root)
    ordinary = adapted = None
    try:
        for index, mode in enumerate(("ordinary", "tofa", "ordinary", "tofa"), 1):
            with route(root, mode, native_catalog, merged, records, routes) as wrapper:
                with contextlib.closing(Engine(str(wrapper), root, [])) as engine:
                    if index in (1, 2):
                        name = "PROTOTYPE ordinary history" if index == 1 else "PROTOTYPE tofa history"
                        response = engine.call("thread/start", {"cwd": str(root / "workspace"), "historyMode": "legacy", "threadSource": "user"})
                        thread_id = response["thread"]["id"]
                        engine.turn(thread_id, name)
                        engine.call("thread/name/set", {"threadId": thread_id, "name": name})
                        threads[thread_id] = name
                        if index == 1:
                            ordinary = thread_id
                        else:
                            adapted = thread_id
                            resumed = engine.call("thread/resume", {"threadId": ordinary})
                            check(resumed["modelProvider"] == "openai" and resumed["model"] == NATIVE,
                                  "native resume changed provider or model")
                            engine.turn(ordinary, "Continue ordinary history while tofa is active")
                    if index == 3:
                        response = engine.response("thread/resume", {"threadId": adapted})
                        check("Model provider `nebius-tofa` not found" in response.get("error", {}).get("message", ""),
                              "ordinary mode did not reject absent provider explicitly")
                    if index == 4:
                        resumed = engine.call("thread/resume", {"threadId": adapted})
                        check(resumed["modelProvider"] == "nebius-tofa" and resumed["model"] == MODEL,
                              "tofa relaunch changed recorded routing")
                        engine.turn(adapted, "Continue tofa history after restoring its provider")
                    state = read_state(engine, threads)
                    phases.append({"phase": index, "mode": mode, "sessions": state})
                    emit("history", **phases[-1])
                if open_desktop:
                    desktop_phases.append(desktop(root, wrapper, mode, index, args.desktop_smoke))
                if snapshot(root) != baseline:
                    config = (root / "codex/config.toml").read_text()
                    tool_env = config.partition("[mcp_servers.node_repl.env]\n")[2].split("\n[", 1)[0]
                    persisted = "CODEX_CLI_PATH = " + json.dumps(str(wrapper)) in tool_env.splitlines()
                    failure = {"issue": 34, "throwaway": True, "result": "failed_shared_config_preservation",
                               "engine_version": version, "actual_desktop_opened": open_desktop,
                               "real_user_profile_used": False, "phase": index, "mode": mode,
                               "desktop_phases": desktop_phases, "ui_verified": False,
                               "auth_unchanged": snapshot(root)["auth.json"] == baseline["auth.json"],
                               "config_unchanged": snapshot(root)["config.toml"] == baseline["config.toml"],
                               "temporary_executable_persisted_at": "mcp_servers.node_repl.env.CODEX_CLI_PATH" if persisted else None,
                               "temporary_executable_removed_on_cleanup": persisted,
                               "native_model_unchanged": 'model = "' + NATIVE + '"' in config.splitlines(),
                               "native_provider_unchanged": 'model_provider = "openai"' in config.splitlines(),
                               "phases": phases, "requests": records,
                               "verdict": "The desktop persists its temporary executable into shared tool settings. This wrapper is not ready for the ordinary profile."}
                    args.report.parent.mkdir(parents=True, exist_ok=True)
                    args.report.write_text(json.dumps(failure, indent=2) + "\n")
                    emit("qualification_failed", report=str(args.report), reason=failure["verdict"])
                    raise RuntimeError("shared config/auth contents changed; see sanitized evidence")
            for expired in routes:
                with socket.socket() as probe:
                    check(probe.connect_ex(("127.0.0.1", int(expired["endpoint"].rsplit(":", 1)[1]))) != 0,
                          "expired route is still listening")
        check(len({r["endpoint"] for r in routes}) == 2 and routes[0]["token"] != routes[1]["token"], "route was reused")
        metadata_files = [path for path in (root / "codex").rglob("*") if path.is_file() and path.name not in ("auth.json", "config.toml")]
        leaked = any(route_info["token"].encode() in path.read_bytes() for route_info in routes for path in metadata_files)
        check(not leaked, "temporary bearer appeared in shared engine state")
        report = {"issue": 34, "throwaway": True, "engine_version": version,
                  "engine_sha256": hashlib.sha256(ENGINE.read_bytes()).hexdigest(),
                  "actual_desktop_opened": open_desktop, "real_user_profile_used": False,
                  "paid_inference_requests": 0, "shared_scratch_config_and_auth_unchanged": snapshot(root) == baseline,
                  "native_bundled_descriptors_preserved": True, "fresh_tofa_routes": len(routes),
                  "expired_routes_closed": True, "local_bearers_absent_from_shared_engine_state": not leaked,
                  "phases": phases, "desktop_phases": desktop_phases, "requests": records,
                  "limitations": ["bundled native catalog snapshot, not live account catalog", "desktop UI observations require a human",
                                  "native auxiliary requests under the tofa provider are rejected, not remapped",
                                  "normal user profile, onboarding, managed policy, upgrades and crashes not qualified"]}
        args.report.parent.mkdir(parents=True, exist_ok=True)
        args.report.write_text(json.dumps(report, indent=2) + "\n")
        emit("complete", report=str(args.report), scratch_retained=str(root))
    finally:
        native.close()


if __name__ == "__main__":
    main()
