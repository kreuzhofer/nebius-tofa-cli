"""Opt-in installed-engine checks for #34; synthetic state and loopback only.

TOFA_TEST_DESKTOP_ENGINE=/Applications/ChatGPT.app/Contents/Resources/codex \
    python3 scripts/desktop_history_test.py -v

These checks exercise the engine contract, not desktop UI parity or the launcher.
"""

import contextlib
import http.server
import json
import os
from pathlib import Path
import queue
import subprocess
import tempfile
import threading
import time
import unittest


class Engine:
    def __init__(self, executable, root, overrides):
        env = {key: os.environ[key] for key in ("PATH", "TMPDIR") if key in os.environ}
        env.update(HOME=str(root), CODEX_HOME=str(root / "codex"),
                   OTEL_SDK_DISABLED="true", TOFA_HISTORY_FIXTURE_KEY="synthetic")
        args = [executable, "app-server"]
        for override in overrides:
            args.extend(["-c", override])
        self.errors = tempfile.TemporaryFile(mode="w+t")
        self.process = subprocess.Popen(
            args, cwd=root / "workspace", env=env, stdin=subprocess.PIPE,
            stdout=subprocess.PIPE, stderr=self.errors, text=True,
        )
        self.messages = queue.Queue()
        self.sequence = 0
        self.reader = threading.Thread(target=self._read, daemon=True)
        self.reader.start()
        try:
            self.call("initialize", {"clientInfo": {"name": "tofa_history_fixture", "version": "1"},
                                     "capabilities": {"experimentalApi": True}})
            self.send({"method": "initialized"})
        except BaseException:
            self.close()
            raise

    def _read(self):
        try:
            for line in self.process.stdout:
                self.messages.put(json.loads(line))
        finally:
            self.messages.put(None)

    def send(self, message):
        self.process.stdin.write(json.dumps(message) + "\n")
        self.process.stdin.flush()

    def receive(self, deadline):
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise TimeoutError("fixture engine deadline exceeded")
        try:
            message = self.messages.get(timeout=remaining)
        except queue.Empty as error:
            raise TimeoutError("fixture engine deadline exceeded") from error
        if message is None:
            self.errors.seek(0)
            raise RuntimeError("fixture engine exited before responding: " + self.errors.read())
        if "method" in message and "id" in message:
            raise RuntimeError("unexpected engine callback: " + message["method"])
        return message

    def response(self, method, params, deadline=None):
        if deadline is None:
            deadline = time.monotonic() + 30
        self.sequence += 1
        self.send({"id": self.sequence, "method": method, "params": params})
        while True:
            message = self.receive(deadline)
            if message.get("id") == self.sequence:
                return message

    def call(self, method, params, deadline=None):
        message = self.response(method, params, deadline)
        if "error" in message:
            raise RuntimeError(method + ": " + message["error"]["message"])
        return message["result"]

    def turn(self, thread_id, text):
        deadline = time.monotonic() + 30
        self.call("turn/start", {"threadId": thread_id, "input": [{"type": "text", "text": text}]}, deadline)
        while True:
            message = self.receive(deadline)
            if message.get("method") == "turn/completed" and message["params"]["threadId"] == thread_id:
                turn = message["params"]["turn"]
                if turn["status"] != "completed":
                    raise RuntimeError("synthetic turn failed: " + json.dumps(turn.get("error")))
                return

    def close(self):
        self.process.stdin.close()
        try:
            self.process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            self.process.kill()
            self.process.wait(timeout=5)
        self.reader.join(timeout=5)
        self.process.stdout.close()
        self.errors.close()


@unittest.skipUnless(os.environ.get("TOFA_TEST_DESKTOP_ENGINE"), "set TOFA_TEST_DESKTOP_ENGINE")
class DesktopHistoryContract(unittest.TestCase):
    def setUp(self):
        self.engine = os.environ["TOFA_TEST_DESKTOP_ENGINE"]
        self.assertTrue(Path(self.engine).is_absolute())
        self.temp = tempfile.TemporaryDirectory(prefix="tofa-history-test-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        for name in ("codex", "workspace"):
            (self.root / name).mkdir()
        version = subprocess.check_output([self.engine, "--version"], cwd=self.root,
                                          env={"HOME": str(self.root), "CODEX_HOME": str(self.root / "codex")},
                                          text=True, timeout=5).strip()
        self.assertIn(version, ("codex-cli 0.155.0-alpha.9.2", "codex-cli 0.155.0-alpha.16.3",
                                    "codex-cli 0.155.0-alpha.16.4"),
                      "requalify the history contract for a changed engine")
        # Synthetic API-key login in the owned temporary home only.
        (self.root / "codex" / "auth.json").write_text(json.dumps({"OPENAI_API_KEY": "synthetic"}))
        self.requests = []
        requests = self.requests

        class Provider(http.server.BaseHTTPRequestHandler):
            def log_message(self, *_args):
                pass

            def do_POST(self):
                payload = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                requests.append({"path": self.path, "model": payload["model"],
                                 "authorization": self.headers.get("Authorization")})
                item = {"id": "msg_fixture", "type": "message", "role": "assistant", "status": "completed",
                        "content": [{"type": "output_text", "text": "Synthetic history answer.", "annotations": []}]}
                response = {"id": "resp_fixture", "status": "completed", "output": [item],
                            "usage": {"input_tokens": 10, "output_tokens": 4, "total_tokens": 14}}
                events = [
                    ("response.created", {"response": {"id": "resp_fixture", "status": "in_progress", "output": []}}),
                    ("response.output_item.added", {"output_index": 0, "item": item}),
                    ("response.output_item.done", {"output_index": 0, "item": item}),
                    ("response.completed", {"response": response}),
                ]
                body = "".join("event: " + kind + "\ndata: " + json.dumps(dict(value, type=kind)) + "\n\n"
                               for kind, value in events).encode()
                self.send_response(200)
                self.send_header("Content-Type", "text/event-stream")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

        server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Provider)
        worker = threading.Thread(target=server.serve_forever, daemon=True)
        worker.start()
        def stop_server():
            server.shutdown()
            server.server_close()
            worker.join(timeout=5)
        self.addCleanup(stop_server)
        endpoint = "http://127.0.0.1:" + str(server.server_port)
        self.provider = ('{name="Synthetic provider",base_url=' + json.dumps(endpoint) +
                         ',env_key="TOFA_HISTORY_FIXTURE_KEY",wire_api="responses",'
                         'requires_openai_auth=false,supports_websockets=false,request_max_retries=0,stream_max_retries=0}')
        self.base = ['model="fixture-native"', 'model_provider="openai"',
                     'openai_base_url=' + json.dumps(endpoint),
                     'approval_policy="never"', 'sandbox_mode="read-only"',
                     'web_search="disabled"', 'cli_auth_credentials_store="file"',
                     'features.shell_snapshot=false', 'features.analytics=false']
        self.tofa = ['model="moonshotai/Kimi-K3"', 'model_provider="nebius-tofa"',
                     'model_providers.nebius-tofa=' + self.provider]

    @contextlib.contextmanager
    def launch(self, tofa=False):
        engine = Engine(self.engine, self.root, self.base + (self.tofa if tofa else []))
        try:
            yield engine
        finally:
            engine.close()

    def create(self, engine, name):
        result = engine.call("thread/start", {"cwd": str(self.root / "workspace"), "historyMode": "legacy"})
        thread_id = result["thread"]["id"]
        engine.turn(thread_id, name)
        engine.call("thread/name/set", {"threadId": thread_id, "name": name})
        return thread_id

    def read_history(self, engine, thread_id, user_messages, assistant_messages=None):
        thread = engine.call("thread/read", {"threadId": thread_id, "includeTurns": True})["thread"]
        self.assertEqual(len(thread["turns"]), len(user_messages))
        items = [item for turn in thread["turns"] for item in turn["items"]]
        self.assertEqual([part["text"] for item in items if item["type"] == "userMessage"
                          for part in item["content"] if part["type"] == "text"], user_messages)
        self.assertEqual([item["text"] for item in items if item["type"] == "agentMessage"],
                         assistant_messages if assistant_messages is not None else
                         ["Synthetic history answer."] * len(user_messages))
        return thread

    def test_engine_history_preserves_explicit_provider_change(self):
        with self.launch() as engine:
            ordinary = self.create(engine, "Ordinary fixture conversation")
        with self.launch(tofa=True) as engine:
            listed = engine.call("thread/list", {"modelProviders": [], "useStateDbOnly": True})["data"]
            self.assertEqual([row["id"] for row in listed], [ordinary])
            resumed = engine.call("thread/resume", {"threadId": ordinary})
            self.assertEqual((resumed["modelProvider"], resumed["model"]), ("openai", "fixture-native"))
            engine.turn(ordinary, "Continue ordinary conversation in tofa mode")
            adapted = self.create(engine, "Token Factory fixture conversation")
        with self.launch() as engine:
            listed = engine.call("thread/list", {"modelProviders": [], "useStateDbOnly": True})["data"]
            self.assertCountEqual([row["id"] for row in listed], [ordinary, adapted])
            for thread_id, name, provider, messages in [
                (ordinary, "Ordinary fixture conversation", "openai",
                 ["Ordinary fixture conversation", "Continue ordinary conversation in tofa mode"]),
                (adapted, "Token Factory fixture conversation", "nebius-tofa", ["Token Factory fixture conversation"]),
            ]:
                thread = self.read_history(engine, thread_id, messages)
                self.assertEqual((thread["id"], thread["name"], thread["modelProvider"], thread["cwd"]),
                                 (thread_id, name, provider, str(self.root / "workspace")))
            # This is the inspected desktop's resume shape. The passing assertion
            # records the accepted unavailable-provider limitation, not UI acceptance.
            request_count = len(self.requests)
            failed = engine.response("thread/resume", {"threadId": adapted, "model": None, "modelProvider": None})
            self.assertIn("Model provider `nebius-tofa` not found", failed["error"]["message"])
            self.assertEqual(len(self.requests), request_count, "missing provider silently sent inference")
            # The API has a recovery mechanism the inspected desktop UI lacks.
            resumed = engine.call("thread/resume", {"threadId": adapted, "model": "fixture-native", "modelProvider": "openai"})
            self.assertEqual(resumed["thread"]["id"], adapted)
            self.assertEqual((resumed["modelProvider"], resumed["model"]), ("openai", "fixture-native"))
            engine.turn(adapted, "Explicitly continue with the available fixture provider")
        with self.launch(tofa=True) as engine:
            resumed = engine.call("thread/resume", {"threadId": adapted})
            self.assertEqual((resumed["thread"]["id"], resumed["modelProvider"], resumed["model"]),
                             (adapted, "openai", "fixture-native"))
            thread = self.read_history(engine, adapted, ["Token Factory fixture conversation",
                                                       "Explicitly continue with the available fixture provider"])
            self.assertEqual(thread["name"], "Token Factory fixture conversation")
            listed = engine.call("thread/list", {"modelProviders": [], "useStateDbOnly": True})["data"]
            self.assertCountEqual([row["id"] for row in listed], [ordinary, adapted])
        self.assertEqual([request["model"] for request in self.requests],
                         ["fixture-native", "fixture-native", "moonshotai/Kimi-K3", "fixture-native"])
        self.assertTrue(all(request["path"] == "/responses" and request["authorization"] == "Bearer synthetic"
                            for request in self.requests))

    def test_tofa_history_resumes_when_provider_returns(self):
        with self.launch(tofa=True) as engine:
            thread_id = self.create(engine, "Relaunch fixture conversation")
        with self.launch() as engine:
            listed = engine.call("thread/list", {"modelProviders": [], "useStateDbOnly": True})["data"]
            self.assertEqual([row["id"] for row in listed], [thread_id])
            thread = self.read_history(engine, thread_id, ["Relaunch fixture conversation"])
            self.assertEqual(thread["name"], "Relaunch fixture conversation")
            failed = engine.response("thread/resume", {"threadId": thread_id})
            self.assertIn("Model provider `nebius-tofa` not found", failed["error"]["message"])
            self.assertEqual(len(self.requests), 1)
        with self.launch(tofa=True) as engine:
            resumed = engine.call("thread/resume", {"threadId": thread_id})
            self.assertEqual((resumed["thread"]["id"], resumed["modelProvider"], resumed["model"]),
                             (thread_id, "nebius-tofa", "moonshotai/Kimi-K3"))
            engine.turn(thread_id, "Continue after relaunching with Token Factory")
        with self.launch() as engine:
            listed = engine.call("thread/list", {"modelProviders": [], "useStateDbOnly": True})["data"]
            self.assertEqual([row["id"] for row in listed], [thread_id])
            thread = self.read_history(engine, thread_id, ["Relaunch fixture conversation",
                                                        "Continue after relaunching with Token Factory"])
            self.assertEqual((thread["name"], thread["modelProvider"], thread["cwd"]),
                             ("Relaunch fixture conversation", "nebius-tofa", str(self.root / "workspace")))
        self.assertEqual([request["model"] for request in self.requests], ["moonshotai/Kimi-K3"] * 2)

    def test_ordinary_history_hydrates_without_live_inference(self):
        with self.launch() as engine:
            ordinary = self.create(engine, "Ordinary round-trip conversation")
        with self.launch(tofa=True) as engine:
            resumed = engine.call("thread/resume", {"threadId": ordinary, "model": None, "modelProvider": None})
            self.assertEqual((resumed["modelProvider"], resumed["model"]), ("openai", "fixture-native"))
            engine.turn(ordinary, "Native continuation during tofa")
            adapted = self.create(engine, "Readable Token Factory conversation")
        with self.launch() as engine:
            failed = engine.response("thread/resume", {"threadId": adapted, "model": None, "modelProvider": None})
            self.assertIn("Model provider `nebius-tofa` not found", failed["error"]["message"])

        # Qualification fixture only: production must install this entry through
        # conflict-aware integration after establishing ordinary-profile ownership.
        inactive = ('[model_providers.nebius-tofa]\nname="Nebius Token Factory"\n'
                    'base_url="http://127.0.0.1:0"\nwire_api="responses"\n'
                    'requires_openai_auth=false\nsupports_websockets=false\n'
                    'request_max_retries=0\nstream_max_retries=0\n'
                    'env_key_instructions="Relaunch through tofa with --model moonshotai/Kimi-K3"\n')
        messages = ["Readable Token Factory conversation"]
        answers = ["Synthetic history answer."]
        for key in ("TOFA_MISSING_LAUNCH_CREDENTIAL", "TOFA_HISTORY_FIXTURE_KEY"):
            with self.subTest(credential=key):
                # The second key is deliberately present in Engine's environment.
                (self.root / "codex" / "config.toml").write_text(inactive + 'env_key=' + json.dumps(key) + '\n')
                before = len(self.requests)
                with self.launch() as engine:
                    listed = engine.call("thread/list", {"modelProviders": [], "useStateDbOnly": True})["data"]
                    self.assertCountEqual([row["id"] for row in listed], [ordinary, adapted])
                    thread = self.read_history(engine, ordinary, ["Ordinary round-trip conversation",
                                                                "Native continuation during tofa"])
                    self.assertEqual((thread["name"], thread["modelProvider"], thread["cwd"]),
                                     ("Ordinary round-trip conversation", "openai", str(self.root / "workspace")))
                    thread = self.read_history(engine, adapted, messages, answers)
                    self.assertEqual((thread["name"], thread["modelProvider"], thread["cwd"]),
                                     (messages[0], "nebius-tofa", str(self.root / "workspace")))
                    resumed = engine.call("thread/resume", {"threadId": adapted, "model": None, "modelProvider": None})
                    self.assertEqual((resumed["thread"]["id"], resumed["modelProvider"], resumed["model"]),
                                     (adapted, "nebius-tofa", "moonshotai/Kimi-K3"))
                    if key == "TOFA_MISSING_LAUNCH_CREDENTIAL":
                        with self.assertRaisesRegex(RuntimeError, "synthetic turn failed") as unavailable:
                            engine.turn(adapted, "Attempt while inactive")
                        self.assertIn("Relaunch through tofa", str(unavailable.exception))
                    else:
                        # alpha.16.3 treats connection refusal as network loss
                        # kept retrying for 120 seconds despite retry limits 0.
                        # Record that limitation; do not call it an explicit
                        # terminal failure or let qualification wait forever.
                        turn = engine.call("turn/start", {"threadId": adapted, "input": [
                            {"type": "text", "text": "Attempt while inactive"}]})["turn"]
                        deadline = time.monotonic() + 15
                        while True:
                            message = engine.receive(deadline)
                            if message.get("method") == "error":
                                self.assertEqual(message["params"]["threadId"], adapted)
                                self.assertIn("Connection failed", message["params"]["error"]["additionalDetails"])
                                break
                        engine.call("turn/interrupt", {"threadId": adapted, "turnId": turn["id"]})
                    self.assertEqual(len(self.requests), before, "inactive history reached inference")
                # A failed send is still a recorded user message in the same thread.
                messages.append("Attempt while inactive")
                with self.launch(tofa=True) as engine:
                    resumed = engine.call("thread/resume", {"threadId": adapted, "model": None, "modelProvider": None})
                    self.assertEqual((resumed["thread"]["id"], resumed["modelProvider"], resumed["model"]),
                                     (adapted, "nebius-tofa", "moonshotai/Kimi-K3"))
                    engine.turn(adapted, "Continue after fresh launch")
                messages.append("Continue after fresh launch")
                answers.append("Synthetic history answer.")
                self.assertEqual(len(self.requests), before + 1)
        with self.launch() as engine:
            self.read_history(engine, adapted, messages, answers)
            listed = engine.call("thread/list", {"modelProviders": [], "useStateDbOnly": True})["data"]
            self.assertCountEqual([row["id"] for row in listed], [ordinary, adapted])


if __name__ == "__main__":
    unittest.main()
