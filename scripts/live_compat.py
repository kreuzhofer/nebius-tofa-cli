"""Opt-in prototype qualification through the real launcher and installed Codex.

Uses saved launcher credentials. Runs three scratch coding sessions by default.
The Codex shim observes SSE between the client and the launcher's loopback adapter;
it never receives the Nebius key. Only counts/status/timing leave scratch storage.
macOS/Linux only; Python is a developer-test dependency, not a launcher dependency.
"""
import argparse
import hashlib
import hmac
import http.client
import http.server
import json
import os
from pathlib import Path
import platform
import re
import shlex
import shutil
import signal
import subprocess
import sys
import tempfile
import threading
import time
from urllib.parse import urlsplit

MODEL = "moonshotai/Kimi-K3"
INITIAL = """Use a shell tool to read input.json in this workspace. Create summary.json
with exactly the numeric fields count, total and max calculated from its numbers.
Use a shell tool to validate the result with Python assertions. Do not edit input.json.
Work only in this workspace; do not inspect environment variables or credentials.
Finish with a short explanation of the result. Do not install dependencies."""
FOLLOWUP = """Continue the previous task. Extend summary.json with min and average,
keeping the existing fields. Use a shell tool to verify all five values with Python
assertions against input.json. Work only in this workspace. Finish with a short
explanation. Do not install dependencies or inspect credentials/environment variables."""
EXPECTED = [{"count": 4, "total": 18, "max": 9},
            {"count": 4, "total": 18, "max": 9, "min": -2, "average": 4.5}]


def write_json(path, value):
    with open(path, "x", encoding="utf-8") as stream:
        os.chmod(path, 0o600)
        json.dump(value, stream, indent=2)
        stream.write("\n")


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest() if path.is_file() else None


def observe(args):
    """Test-only Codex executable shim; forwards only to the supplied local adapter."""
    index = next(i for i, arg in enumerate(args) if arg.startswith("model_providers.nebius-tofa="))
    match = re.search(r'base_url\s*=\s*("[^"]+")', args[index])
    endpoint = json.loads(match.group(1))
    target = urlsplit(endpoint)
    if (target.scheme != "http" or target.hostname != "127.0.0.1" or not target.port
            or target.path or target.query or target.fragment or target.username):
        raise ValueError("expected the launcher's loopback adapter")
    token = os.environ["TOFA_API_KEY"]
    observations = []

    class Observer(http.server.BaseHTTPRequestHandler):
        def log_message(self, *unused):
            pass

        def do_POST(self):
            if not hmac.compare_digest(self.headers.get("Authorization", ""), "Bearer " + token):
                self.send_error(401)
                return
            if self.path != "/responses":
                self.send_error(404)
                return
            if len(observations) >= 12:
                self.send_error(429)
                return
            length = int(self.headers.get("Content-Length", "0"))
            if not 0 < length <= 16 * 1024 * 1024:
                self.send_error(413)
                return
            record = {"status": 0, "text_deltas": 0, "tool_deltas": 0,
                      "completed": False, "first_delta_ms": None, "completed_ms": None}
            observations.append(record)
            start = time.monotonic()
            connection = http.client.HTTPConnection(target.hostname, target.port, timeout=90)
            stage = "adapter_request"
            try:
                connection.request("POST", "/responses", self.rfile.read(length),
                                   {"Authorization": "Bearer " + token,
                                    "Content-Type": "application/json", "Accept": "text/event-stream"})
                response = connection.getresponse()
                record["status"] = response.status
                self.send_response(response.status)
                self.send_header("Content-Type", response.getheader("Content-Type", "application/octet-stream"))
                self.end_headers()
                while True:
                    stage = "adapter_read"
                    line = response.readline(1024 * 1024)
                    if not line:
                        break
                    stage = "client_write"
                    self.wfile.write(line)
                    self.wfile.flush()
                    if line.startswith(b"data: "):
                        try:
                            event = json.loads(line[6:])
                        except (ValueError, UnicodeDecodeError):
                            continue
                        kind = event.get("type") if isinstance(event, dict) else None
                        if (kind in ("response.output_text.delta", "response.function_call_arguments.delta")
                                and isinstance(event.get("delta"), str) and event["delta"]
                                and not record["completed"]):
                            field = "text_deltas" if kind == "response.output_text.delta" else "tool_deltas"
                            record[field] += 1
                            if record["first_delta_ms"] is None:
                                record["first_delta_ms"] = round((time.monotonic() - start) * 1000, 3)
                        if kind == "response.completed":
                            record["completed"] = True
                            record["completed_ms"] = round((time.monotonic() - start) * 1000, 3)
            except (OSError, http.client.HTTPException) as error:
                if stage == "client_write" and record["completed"] and isinstance(error, (BrokenPipeError, ConnectionResetError)):
                    record["client_closed_after_completion"] = True
                else:
                    record["transport_error"] = True
                    record["error_stage"] = stage
                    record["error_kind"] = "timeout" if isinstance(error, TimeoutError) else "connection"
            finally:
                connection.close()

    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Observer)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    replacement = json.dumps(f"http://127.0.0.1:{server.server_port}")
    args[index] = args[index][:match.start(1)] + replacement + args[index][match.end(1):]
    # The launcher keeps its normal HOME for credential storage. Only the actual
    # client gets the scratch HOME, configuration and allowlisted environment.
    env = {name: os.environ[name] for name in ("PATH", "TMPDIR", "LANG", "LC_ALL") if name in os.environ}
    env.update(HOME=os.environ["TOFA_LIVE_HOME"], CODEX_HOME=os.environ["TOFA_LIVE_CODEX_HOME"],
               TOFA_API_KEY=token, OTEL_SDK_DISABLED="true")
    try:
        result = subprocess.call([os.environ["TOFA_LIVE_CODEX"]] + args, env=env)
    finally:
        server.shutdown()
        server.server_close()
        write_json(os.environ["TOFA_LIVE_OBSERVATIONS"], observations)
    return result


def stop(process):
    try:
        os.killpg(process.pid, signal.SIGTERM)
        process.wait(timeout=3)
    except subprocess.TimeoutExpired:
        os.killpg(process.pid, signal.SIGKILL)
        process.wait(timeout=3)
    except ProcessLookupError:
        pass


def turn(command, env, workspace, prompt, timeout):
    """Consume client output without retaining conversation text or launcher project IDs."""
    summary = {"exit_code": None, "tools_succeeded": 0, "turn_completed": False,
               "client_error": False, "metadata_warning": False, "timed_out": False}
    thread_ids = []
    process = subprocess.Popen(command, cwd=workspace, env=env, stdin=subprocess.PIPE,
                               stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)

    def consume(stream, events):
        size = 0
        for line in iter(lambda: stream.readline(1024 * 1024), b""):
            size += len(line)
            if size > 8 * 1024 * 1024:
                summary["output_limit"] = True
                os.killpg(process.pid, signal.SIGTERM)
                break
            if b"Model metadata for" in line:
                summary["metadata_warning"] = True
            if not events:
                continue
            try:
                event = json.loads(line)
            except (ValueError, UnicodeDecodeError):
                continue
            if not isinstance(event, dict):
                continue
            kind = event.get("type")
            if kind == "thread.started":
                identity = event.get("thread_id", "")
                if re.fullmatch(r"[a-fA-F0-9-]{36}", identity):
                    thread_ids.append(identity)
            if kind == "turn.completed":
                summary["turn_completed"] = True
            if kind in ("error", "turn.failed"):
                summary["client_error"] = True
            item = event.get("item", {})
            if (kind == "item.completed" and item.get("type") == "command_execution"
                    and item.get("status") == "completed" and item.get("exit_code") == 0):
                summary["tools_succeeded"] += 1

    readers = [threading.Thread(target=consume, args=(process.stdout, True), daemon=True),
               threading.Thread(target=consume, args=(process.stderr, False), daemon=True)]
    for reader in readers:
        reader.start()
    try:
        process.stdin.write(prompt.encode())
        process.stdin.close()
        process.wait(timeout=timeout)
    except subprocess.TimeoutExpired:
        summary["timed_out"] = True
        stop(process)
    except BaseException:
        stop(process)
        raise
    finally:
        for reader in readers:
            reader.join(timeout=4)
        process.stdout.close()
        process.stderr.close()
    summary["exit_code"] = process.returncode
    return summary, thread_ids


def run_one(options, root):
    workspace = root / "workspace"
    workspace.mkdir()
    client_home = root / "codex-home"
    client_home.mkdir()
    scratch_home = root / "home"
    scratch_home.mkdir()
    config = client_home / "config.toml"
    config.write_text('allow_login_shell = false\n\n[projects.' + json.dumps(str(workspace))
                      + ']\ntrust_level = "trusted"\n')
    before = digest(config)
    (workspace / "input.json").write_text('{"numbers":[4,-2,7,9]}\n')
    original_input = digest(workspace / "input.json")
    shim = root / "bin"
    shim.mkdir()
    shim_path = shim / "codex"
    shim_path.write_text("#!/bin/sh\nexec " + shlex.quote(sys.executable) + " "
                         + shlex.quote(str(Path(__file__).resolve())) + ' --observe "$@"\n')
    shim_path.chmod(0o700)
    env = dict(os.environ)
    env.update(PATH=str(shim) + os.pathsep + os.environ.get("PATH", ""),
               TOFA_LIVE_HOME=str(scratch_home), TOFA_LIVE_CODEX_HOME=str(client_home),
               TOFA_LIVE_CODEX=options.codex)
    base = [options.launcher, "launch", "codex", "--model", MODEL, "--allow-unverified", "--",
            "--ask-for-approval", "never", "--sandbox", "workspace-write", "exec"]
    turns = []
    session = None
    same_session = False
    for number, prompt in enumerate((INITIAL, FOLLOWUP)):
        print(f"  Turn {number + 1}/2", flush=True)
        observation_path = root / f"stream-{number}.json"
        env["TOFA_LIVE_OBSERVATIONS"] = str(observation_path)
        args = (["resume", session] if number else []) + ["--skip-git-repo-check", "--json", "-"]
        result, identities = turn(base + args, env, workspace, prompt, options.timeout)
        if number == 0:
            session = identities[0] if len(identities) == 1 else None
        else:
            same_session = identities == [session]
        try:
            result["files_correct"] = (json.loads((workspace / "summary.json").read_text()) == EXPECTED[number]
                                       and digest(workspace / "input.json") == original_input)
        except (OSError, ValueError):
            result["files_correct"] = False
        try:
            streams = json.loads(observation_path.read_text())
        except (OSError, ValueError):
            streams = []
        result["streams"] = streams
        result["streaming_observed"] = any(
            s["status"] == 200 and s["completed"] and s["text_deltas"] >= 2
            and s["first_delta_ms"] < s["completed_ms"] for s in streams)
        result["passed"] = (result["exit_code"] == 0 and result["turn_completed"]
                            and result["tools_succeeded"] > 0 and result["files_correct"]
                            and result["streaming_observed"] and not result["client_error"]
                            and not result["metadata_warning"] and not result["timed_out"]
                            and not result.get("output_limit", False)
                            and all(s["status"] == 200 and s["completed"] and not s.get("transport_error") for s in streams))
        turns.append(result)
        print("  Turn passed" if result["passed"] else "  Turn failed", flush=True)
        if not session or result["exit_code"] != 0:
            break
    config_preserved = digest(config) == before
    auth_absent = not (client_home / "auth.json").exists()
    preserved = config_preserved and auth_absent
    return {"turns": turns, "same_session": same_session, "scratch_config_preserved": config_preserved,
            "scratch_auth_absent": auth_absent, "scratch_settings_preserved": preserved,
            "passed": len(turns) == 2 and same_session and preserved and all(t["passed"] for t in turns)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--launcher", required=True, help="built tofa executable; reads saved credentials")
    parser.add_argument("--codex", default=shutil.which("codex"))
    parser.add_argument("--runs", type=int, default=3)
    parser.add_argument("--timeout", type=int, default=180, help="seconds per turn")
    parser.add_argument("--output", required=True, help="new sanitized JSON evidence file")
    options = parser.parse_args()
    if os.name != "posix" or not 1 <= options.runs <= 10 or not 1 <= options.timeout <= 600:
        parser.error("requires macOS/Linux, 1–10 runs and a 1–600 second turn timeout")
    for name in ("launcher", "codex"):
        value = getattr(options, name)
        if not value or not Path(value).is_file() or not os.access(value, os.X_OK):
            parser.error(f"{name} must be an executable file")
        setattr(options, name, str(Path(value).resolve()))
    output = Path(options.output).resolve()
    if output.exists() or not output.parent.is_dir():
        parser.error("output must be a new file in an existing directory")
    codex_home = Path(os.environ.get("CODEX_HOME", str(Path.home() / ".codex")))
    launcher_home = Path(os.environ.get("XDG_CONFIG_HOME", str(Path.home() / ".config"))) / "tofa"
    watched = {"codex_config": codex_home / "config.toml", "codex_auth": codex_home / "auth.json",
               "launcher_config": launcher_home / "config.yml", "launcher_file_credential": launcher_home / "credentials.yml"}
    before = {name: digest(path) for name, path in watched.items()}
    evidence = {"model": MODEL, "route": "adapted with test-only loopback SSE observer",
                "platform": platform.system() + "/" + platform.machine(),
                "os_release": platform.release(),
                "started_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
                "launcher_sha256": digest(Path(options.launcher)),
                "harness_sha256": digest(Path(__file__)), "codex_sha256": digest(Path(options.codex)), "runs": []}
    with tempfile.TemporaryDirectory(prefix="tofa-version-") as directory:
        probe_root = Path(directory).resolve()
        (probe_root / "codex").mkdir()
        probe_env = {name: os.environ[name] for name in ("PATH", "TMPDIR", "LANG", "LC_ALL") if name in os.environ}
        probe_env.update(HOME=str(probe_root), CODEX_HOME=str(probe_root / "codex"))
        for name in ("launcher", "codex"):
            result = subprocess.run([getattr(options, name), "--version"], env=probe_env,
                                    capture_output=True, text=True, timeout=10)
            if result.returncode != 0:
                raise RuntimeError("version probe failed")
            evidence[name + "_version"] = result.stdout.strip()
    try:
        for number in range(options.runs):
            print(f"Live run {number + 1}/{options.runs}: {MODEL}", flush=True)
            with tempfile.TemporaryDirectory(prefix="tofa-live-") as directory:
                result = run_one(options, Path(directory).resolve())
            evidence["runs"].append(result)
            print("PASS" if result["passed"] else "FAIL", flush=True)
    finally:
        evidence["normal_files_preserved"] = {name: before[name] == digest(path) for name, path in watched.items()}
        evidence["normal_settings_preserved"] = all(evidence["normal_files_preserved"].values())
        evidence["passed"] = (len(evidence["runs"]) == options.runs and evidence["normal_settings_preserved"]
                              and all(run["passed"] for run in evidence["runs"]))
        write_json(output, evidence)
        print("Qualification passed" if evidence["passed"] else "Qualification failed; inspect the sanitized report", flush=True)
    return 0 if evidence["passed"] else 1


if __name__ == "__main__":
    try:
        sys.exit(observe(sys.argv[2:]) if sys.argv[1:2] == ["--observe"] else main())
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError, StopIteration, KeyError):
        # Exceptions may contain raw argv, paths or provider output. Keep them private.
        print("Live harness failed; see sanitized evidence if available.", file=sys.stderr)
        sys.exit(1)
